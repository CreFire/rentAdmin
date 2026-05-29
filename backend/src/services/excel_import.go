package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"rentadmin/src/models"

	"github.com/xuri/excelize/v2"
)

var yearPattern = regexp.MustCompile(`(20\d{2})`)
var fullDatePattern = regexp.MustCompile(`(20\d{2})[^\d]{0,3}(\d{1,2})[^\d]{0,3}(\d{1,2})`)

type ExcelImportSummary struct {
	ProcessedSheets int      `json:"processedSheets"`
	Inserted        int      `json:"inserted"`
	Updated         int      `json:"updated"`
	Skipped         int      `json:"skipped"`
	Errors          []string `json:"errors"`
	DebugJSONPath   string   `json:"debugJsonPath,omitempty"`
}

type sheetMetadata struct {
	roomNumber       string
	name             string
	phone            string
	idCard           string
	checkInDate      string
	rentCycle        string
	defaultRent      float64
	defaultYear      int
	defaultMonthHint int
}

type rowMapping struct {
	month              int
	waterReading       *float64
	electricityReading *float64
	waterBill          float64
	electricityBill    float64
	extraFee           float64
	totalAmount        float64
	amountPaid         float64
}

type existingTenant struct {
	id                 string
	name               string
	phone              string
	idCard             *string
	checkInDate        *string
	deposit            *float64
	waterReading       float64
	electricityReading float64
}

type importDebugSnapshot struct {
	ExcelPath string             `json:"excelPath"`
	Generated string             `json:"generatedAt"`
	Sheets    []sheetDebugRecord `json:"sheets"`
}

type sheetDebugRecord struct {
	SheetName  string           `json:"sheetName"`
	Skipped    bool             `json:"skipped"`
	SkipReason string           `json:"skipReason,omitempty"`
	RowCount   int              `json:"rowCount"`
	HeaderRow  []string         `json:"headerRow,omitempty"`
	Metadata   *sheetMetadata   `json:"metadata,omitempty"`
	ParsedRows []rowDebugRecord `json:"parsedRows,omitempty"`
}

type rowDebugRecord struct {
	RowNumber          int      `json:"rowNumber"`
	Raw                []string `json:"raw"`
	Accepted           bool     `json:"accepted"`
	SkipReason         string   `json:"skipReason,omitempty"`
	Month              int      `json:"month,omitempty"`
	Date               string   `json:"date,omitempty"`
	WaterReading       *float64 `json:"waterReading,omitempty"`
	ElectricityReading *float64 `json:"electricityReading,omitempty"`
	WaterBill          float64  `json:"waterBill,omitempty"`
	ElectricityBill    float64  `json:"electricityBill,omitempty"`
	ExtraFee           float64  `json:"extraFee,omitempty"`
	TotalAmount        float64  `json:"totalAmount,omitempty"`
	AmountPaid         float64  `json:"amountPaid,omitempty"`
	Operation          string   `json:"operation,omitempty"`
}

func ImportTenantsFromExcel(db *sql.DB, providedPath string) (*ExcelImportSummary, error) {
	excelPath, err := resolveExcelPath(providedPath)
	if err != nil {
		return nil, err
	}

	workbook, err := excelize.OpenFile(excelPath)
	if err != nil {
		return nil, fmt.Errorf("open excel file: %w", err)
	}
	defer workbook.Close()

	summary := &ExcelImportSummary{}
	now := time.Now().UTC().Format(time.RFC3339)
	debugSnapshot := importDebugSnapshot{
		ExcelPath: excelPath,
		Generated: now,
		Sheets:    make([]sheetDebugRecord, 0),
	}

	for _, sheetName := range workbook.GetSheetList() {
		sheetDebug := sheetDebugRecord{
			SheetName: sheetName,
		}
		if shouldSkipSheet(sheetName) {
			summary.Skipped++
			sheetDebug.Skipped = true
			sheetDebug.SkipReason = "sheet name matched skip rules"
			debugSnapshot.Sheets = append(debugSnapshot.Sheets, sheetDebug)
			continue
		}

		rows, err := workbook.GetRows(sheetName)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: read rows failed: %v", sheetName, err))
			sheetDebug.Skipped = true
			sheetDebug.SkipReason = fmt.Sprintf("read rows failed: %v", err)
			debugSnapshot.Sheets = append(debugSnapshot.Sheets, sheetDebug)
			continue
		}
		sheetDebug.RowCount = len(rows)
		if len(rows) < 5 || !isDetailSheet(rows) {
			summary.Skipped++
			sheetDebug.Skipped = true
			sheetDebug.SkipReason = "not a detail sheet or too few rows"
			if len(rows) >= 3 {
				sheetDebug.HeaderRow = rows[2]
			}
			debugSnapshot.Sheets = append(debugSnapshot.Sheets, sheetDebug)
			continue
		}

		headerRow := rows[2]
		sheetDebug.HeaderRow = headerRow
		headerIndex := buildHeaderIndex(headerRow)
		roomColumn, ok := headerIndex["房间号"]
		if !ok {
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: missing 房间号 header", sheetName))
			sheetDebug.Skipped = true
			sheetDebug.SkipReason = "missing 房间号 header"
			debugSnapshot.Sheets = append(debugSnapshot.Sheets, sheetDebug)
			continue
		}

		metadata := parseSheetMetadata(sheetName, rows, roomColumn)
		sheetDebug.Metadata = &metadata
		if metadata.roomNumber == "" {
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: missing room metadata", sheetName))
			sheetDebug.Skipped = true
			sheetDebug.SkipReason = "missing room metadata"
			debugSnapshot.Sheets = append(debugSnapshot.Sheets, sheetDebug)
			continue
		}

		if err := deleteImportedRowsBySheet(db, sheetName); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: delete existing imported rows failed: %v", sheetName, err))
			sheetDebug.Skipped = true
			sheetDebug.SkipReason = fmt.Sprintf("delete existing imported rows failed: %v", err)
			debugSnapshot.Sheets = append(debugSnapshot.Sheets, sheetDebug)
			continue
		}

		summary.ProcessedSheets++
		currentYear := metadata.defaultYear
		lastMonth := 0
		sheetDebug.ParsedRows = make([]rowDebugRecord, 0)

		for rowIndex := 4; rowIndex < len(rows); rowIndex++ {
			row := rows[rowIndex]
			mapping, ok, skipReason := parseBillingRow(row, headerIndex)
			rowDebug := rowDebugRecord{
				RowNumber:  rowIndex + 1,
				Raw:        row,
				Accepted:   ok,
				SkipReason: skipReason,
			}
			if !ok {
				sheetDebug.ParsedRows = append(sheetDebug.ParsedRows, rowDebug)
				continue
			}

			if lastMonth != 0 && mapping.month < lastMonth {
				currentYear++
			}
			lastMonth = mapping.month

			date := fmt.Sprintf("%04d-%02d", currentYear, mapping.month)
			rowDebug.Month = mapping.month
			rowDebug.Date = date
			rowDebug.WaterReading = mapping.waterReading
			rowDebug.ElectricityReading = mapping.electricityReading
			rowDebug.WaterBill = mapping.waterBill
			rowDebug.ElectricityBill = mapping.electricityBill
			rowDebug.ExtraFee = mapping.extraFee
			rowDebug.TotalAmount = mapping.totalAmount
			rowDebug.AmountPaid = mapping.amountPaid
			existingExact, err := getExistingTenant(db, metadata.roomNumber, date)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s[%d]: query existing record failed: %v", sheetName, rowIndex+1, err))
				rowDebug.Accepted = false
				rowDebug.SkipReason = fmt.Sprintf("query existing record failed: %v", err)
				sheetDebug.ParsedRows = append(sheetDebug.ParsedRows, rowDebug)
				continue
			}

			existingLatest, err := getLatestTenantByRoom(db, metadata.roomNumber)
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s[%d]: query latest room record failed: %v", sheetName, rowIndex+1, err))
				rowDebug.Accepted = false
				rowDebug.SkipReason = fmt.Sprintf("query latest room record failed: %v", err)
				sheetDebug.ParsedRows = append(sheetDebug.ParsedRows, rowDebug)
				continue
			}

			record := mergeImportedRow(sheetName, metadata, mapping, existingExact, existingLatest, date, now)
			if existingExact != nil {
				if err := updateImportedTenant(db, existingExact.id, record); err != nil {
					summary.Errors = append(summary.Errors, fmt.Sprintf("%s[%d]: update failed: %v", sheetName, rowIndex+1, err))
					rowDebug.Accepted = false
					rowDebug.SkipReason = fmt.Sprintf("update failed: %v", err)
					sheetDebug.ParsedRows = append(sheetDebug.ParsedRows, rowDebug)
					continue
				}
				rowDebug.Operation = "update"
				summary.Updated++
				sheetDebug.ParsedRows = append(sheetDebug.ParsedRows, rowDebug)
				continue
			}

			if err := insertImportedTenant(db, record); err != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s[%d]: insert failed: %v", sheetName, rowIndex+1, err))
				rowDebug.Accepted = false
				rowDebug.SkipReason = fmt.Sprintf("insert failed: %v", err)
				sheetDebug.ParsedRows = append(sheetDebug.ParsedRows, rowDebug)
				continue
			}
			rowDebug.Operation = "insert"
			summary.Inserted++
			sheetDebug.ParsedRows = append(sheetDebug.ParsedRows, rowDebug)
		}

		debugSnapshot.Sheets = append(debugSnapshot.Sheets, sheetDebug)
	}

	debugJSONPath, err := writeImportDebugJSON(excelPath, debugSnapshot)
	if err != nil {
		summary.Errors = append(summary.Errors, fmt.Sprintf("write debug json failed: %v", err))
	} else {
		summary.DebugJSONPath = debugJSONPath
	}

	return summary, nil
}

func resolveExcelPath(providedPath string) (string, error) {
	candidates := []string{}
	if providedPath != "" {
		candidates = append(candidates, providedPath)
	}

	wd, err := os.Getwd()
	if err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "cent3.xlsx"),
			filepath.Join(wd, "..", "cent3.xlsx"),
			filepath.Join(wd, "cent.xlsx"),
			filepath.Join(wd, "..", "cent.xlsx"),
		)
	}

	executable, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(execDir, "cent3.xlsx"),
			filepath.Join(execDir, "..", "cent3.xlsx"),
			filepath.Join(execDir, "cent.xlsx"),
			filepath.Join(execDir, "..", "cent.xlsx"),
		)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("cent3.xlsx or cent.xlsx not found in expected locations")
}

func shouldSkipSheet(sheetName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(sheetName))
	if normalized == "总表" || normalized == "工作表1" {
		return true
	}
	return strings.Contains(normalized, "old") || strings.Contains(sheetName, "副本")
}

func isDetailSheet(rows [][]string) bool {
	if len(rows) < 3 {
		return false
	}
	headerRow := rows[2]
	return len(headerRow) > 0 && strings.TrimSpace(headerRow[0]) == "月份" && hasCell(headerRow, "房间号")
}

func hasCell(row []string, target string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) == target {
			return true
		}
	}
	return false
}

func buildHeaderIndex(row []string) map[string]int {
	index := make(map[string]int, len(row))
	for columnIndex, cell := range row {
		value := strings.TrimSpace(cell)
		if value != "" {
			index[value] = columnIndex
		}
	}
	return index
}

func parseSheetMetadata(sheetName string, rows [][]string, roomColumn int) sheetMetadata {
	metadata := sheetMetadata{
		rentCycle:   "月度",
		defaultYear: 2026,
	}

	headerRow := rows[2]
	roomValue := getCell(headerRow, roomColumn+1)
	nameValue := getCell(headerRow, roomColumn+2)
	metadata.roomNumber = strings.TrimSpace(roomValue)
	metadata.name = strings.TrimSpace(nameValue)

	if metadata.name == "" {
		metadata.roomNumber, metadata.name = splitCombinedRoomAndName(metadata.roomNumber)
	}

	for rowIndex := 3; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		label := strings.TrimSpace(getCell(row, roomColumn))
		primaryValue := strings.TrimSpace(getCell(row, roomColumn+1))
		secondaryValue := strings.TrimSpace(getCell(row, roomColumn+2))

		switch label {
		case "电话", "电话:":
			metadata.phone = pickFirstNonEmpty(primaryValue, secondaryValue)
		case "身份证":
			metadata.idCard = pickFirstNonEmpty(primaryValue, secondaryValue)
		case "房租":
			metadata.defaultRent = parseFloatValue(primaryValue)
			if metadata.defaultRent == 0 {
				metadata.defaultRent = parseFloatValue(secondaryValue)
			}
		case "收租方式":
			metadata.rentCycle = mapRentCycle(pickFirstNonEmpty(primaryValue, secondaryValue))
		}

		for _, candidate := range []string{primaryValue, secondaryValue} {
			if candidate == "" {
				continue
			}
			if metadata.checkInDate == "" {
				if date, month, year := extractFullDate(candidate); date != "" {
					metadata.checkInDate = date
					if year != 0 {
						metadata.defaultYear = year
						metadata.defaultMonthHint = month
					}
				}
			}
			if metadata.name == "" && isCandidateName(candidate) {
				metadata.name = candidate
			}
		}
	}

	if metadata.defaultMonthHint != 0 {
		firstMonth := findFirstBillingMonth(rows)
		if firstMonth != 0 && firstMonth < metadata.defaultMonthHint {
			metadata.defaultYear++
		}
	}

	if metadata.roomNumber == "" {
		metadata.roomNumber = sheetName
	}

	return metadata
}

func splitCombinedRoomAndName(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	parts := strings.Fields(value)
	if len(parts) >= 2 {
		return parts[0], strings.Join(parts[1:], " ")
	}
	return value, ""
}

func findFirstBillingMonth(rows [][]string) int {
	for rowIndex := 4; rowIndex < len(rows); rowIndex++ {
		month, ok := parseMonthValue(getCell(rows[rowIndex], 0))
		if ok {
			return month
		}
	}
	return 0
}

func isCandidateName(value string) bool {
	if value == "" {
		return false
	}
	if yearPattern.MatchString(value) {
		return false
	}
	if strings.Contains(value, "wxid") {
		return false
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return false
	}
	return true
}

func parseBillingRow(row []string, headerIndex map[string]int) (rowMapping, bool, string) {
	monthColumn, ok := headerIndex["月份"]
	if !ok {
		return rowMapping{}, false, "missing 月份 header"
	}

	month, ok := parseMonthValue(getCell(row, monthColumn))
	if !ok {
		return rowMapping{}, false, "invalid month"
	}

	electricityReading := parseOptionalNonZeroFloat(getCell(row, headerIndex["电起"]))
	if electricityReading == nil {
		return rowMapping{}, false, "电起 is empty or zero"
	}

	mapping := rowMapping{
		month:              month,
		waterReading:       parseOptionalNonZeroFloat(getCell(row, headerIndex["水起"])),
		electricityReading: electricityReading,
		waterBill:          parseFloatValue(getCell(row, headerIndex["水费"])),
		electricityBill:    parseFloatValue(getCell(row, headerIndex["电费"])),
		extraFee:           parseFloatValue(getCell(row, headerIndex["额外"])),
		totalAmount:        parseFloatValue(getCell(row, headerIndex["总计金额"])),
		amountPaid:         parseFloatValue(getCell(row, headerIndex["已收"])),
	}

	return mapping, true, ""
}

func parseMonthValue(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "初始值" {
		return 0, false
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}

	month := int(number)
	if month < 1 || month > 12 {
		return 0, false
	}
	return month, true
}

func mergeImportedRow(sheetName string, metadata sheetMetadata, mapping rowMapping, existingExact *existingTenant, existingLatest *existingTenant, date string, now string) *models.TenantRecord {
	merged := pickTenantRecordSource(existingExact, existingLatest)
	name := metadata.name
	if name == "" {
		name = merged.name
	}
	phone := metadata.phone
	if phone == "" {
		phone = merged.phone
	}

	idCard := metadata.idCard
	if idCard == "" && merged.idCard != nil {
		idCard = *merged.idCard
	}

	checkInDate := metadata.checkInDate
	if checkInDate == "" && merged.checkInDate != nil {
		checkInDate = *merged.checkInDate
	}

	rentAmount := metadata.defaultRent
	if rentAmount == 0 {
		rentAmount = mapping.totalAmount - mapping.waterBill - mapping.electricityBill - mapping.extraFee
	}
	if rentAmount < 0 {
		rentAmount = 0
	}

	totalAmount := mapping.totalAmount
	if totalAmount < 0 {
		totalAmount = 0
	}

	amountPaid := mapping.amountPaid
	if amountPaid < 0 {
		amountPaid = 0
	}
	if amountPaid > totalAmount {
		amountPaid = totalAmount
	}

	status := "待缴"
	if amountPaid >= totalAmount {
		status = "已缴"
	} else if amountPaid > 0 {
		status = "部分缴纳"
	}

	recordedAt := fmt.Sprintf("%s-01", date)
	waterReading := merged.waterReading
	if mapping.waterReading != nil {
		waterReading = *mapping.waterReading
	}
	electricityReading := merged.electricityReading
	if mapping.electricityReading != nil {
		electricityReading = *mapping.electricityReading
	}

	record := &models.TenantRecord{
		ID:                     buildImportID(sheetName, date),
		RoomNumber:             metadata.roomNumber,
		Name:                   name,
		Phone:                  phone,
		RentAmount:             rentAmount,
		WaterReading:           waterReading,
		ElectricityReading:     electricityReading,
		WaterBill:              mapping.waterBill,
		ElectricityBill:        mapping.electricityBill,
		TotalAmount:            totalAmount,
		AmountPaid:             amountPaid,
		RentCycle:              metadata.rentCycle,
		UtilityCycle:           "月度",
		Status:                 status,
		Date:                   date,
		RecordedAt:             &recordedAt,
		MonthlyIncome:          amountPaid,
		AnnualIncome:           amountPaid,
		WaterElecIncome:        mapping.waterBill + mapping.electricityBill,
		MonthlyWaterElecIncome: mapping.waterBill + mapping.electricityBill,
		AnnualWaterElecIncome:  mapping.waterBill + mapping.electricityBill,
	}

	record.CreatedAt = mustParseTime(now)
	record.UpdatedAt = mustParseTime(now)
	if idCard != "" {
		record.IDCard = &idCard
	}
	if checkInDate != "" {
		record.CheckInDate = &checkInDate
	}
	record.Deposit = merged.deposit
	return record
}

func pickTenantRecordSource(primary *existingTenant, fallback *existingTenant) existingTenant {
	if primary != nil {
		return *primary
	}
	if fallback != nil {
		return *fallback
	}
	return existingTenant{}
}

func buildImportID(sheetName string, date string) string {
	replacer := strings.NewReplacer(" ", "", "/", "-", ":", "")
	return fmt.Sprintf("import-%s-%s", replacer.Replace(sheetName), replacer.Replace(date))
}

func buildImportIDPrefix(sheetName string) string {
	replacer := strings.NewReplacer(" ", "", "/", "-", ":", "")
	return fmt.Sprintf("import-%s-", replacer.Replace(sheetName))
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().UTC()
	}
	return parsed
}

func getExistingTenant(db *sql.DB, roomNumber string, date string) (*existingTenant, error) {
	row := db.QueryRow(`SELECT id, name, phone, id_card, check_in_date, deposit, water_reading, electricity_reading FROM tenants WHERE room_number = ? AND date = ?`, roomNumber, date)

	var record existingTenant
	err := row.Scan(&record.id, &record.name, &record.phone, &record.idCard, &record.checkInDate, &record.deposit, &record.waterReading, &record.electricityReading)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func getLatestTenantByRoom(db *sql.DB, roomNumber string) (*existingTenant, error) {
	row := db.QueryRow(`SELECT id, name, phone, id_card, check_in_date, deposit, water_reading, electricity_reading FROM tenants WHERE room_number = ? ORDER BY date DESC LIMIT 1`, roomNumber)

	var record existingTenant
	err := row.Scan(&record.id, &record.name, &record.phone, &record.idCard, &record.checkInDate, &record.deposit, &record.waterReading, &record.electricityReading)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func insertImportedTenant(db *sql.DB, record *models.TenantRecord) error {
	_, err := db.Exec(`INSERT INTO tenants(id, room_number, name, phone, id_card, check_in_date, deposit, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.RoomNumber, record.Name, record.Phone, record.IDCard, record.CheckInDate, record.Deposit, record.RentAmount, record.WaterReading, record.ElectricityReading, record.WaterBill, record.ElectricityBill, record.TotalAmount, record.AmountPaid, record.RentCycle, record.UtilityCycle, record.Status, record.Date, record.RecordedAt, record.CreatedAt.UTC().Format(time.RFC3339), record.UpdatedAt.UTC().Format(time.RFC3339), record.MonthlyIncome, record.AnnualIncome, record.WaterElecIncome, record.MonthlyWaterElecIncome, record.AnnualWaterElecIncome)
	return err
}

func updateImportedTenant(db *sql.DB, id string, record *models.TenantRecord) error {
	_, err := db.Exec(`UPDATE tenants SET name=?, phone=?, id_card=?, check_in_date=?, deposit=?, rent_amount=?, water_reading=?, electricity_reading=?, water_bill=?, electricity_bill=?, total_amount=?, amount_paid=?, rent_cycle=?, utility_cycle=?, status=?, recorded_at=?, updated_at=?, monthly_income=?, annual_income=?, water_elec_income=?, monthly_water_elec_income=?, annual_water_elec_income=? WHERE id = ?`,
		record.Name, record.Phone, record.IDCard, record.CheckInDate, record.Deposit, record.RentAmount, record.WaterReading, record.ElectricityReading, record.WaterBill, record.ElectricityBill, record.TotalAmount, record.AmountPaid, record.RentCycle, record.UtilityCycle, record.Status, record.RecordedAt, record.UpdatedAt.UTC().Format(time.RFC3339), record.MonthlyIncome, record.AnnualIncome, record.WaterElecIncome, record.MonthlyWaterElecIncome, record.AnnualWaterElecIncome, id)
	return err
}

func extractFullDate(value string) (string, int, int) {
	match := fullDatePattern.FindStringSubmatch(value)
	if len(match) != 4 {
		return "", 0, 0
	}

	year, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return "", 0, 0
	}

	return fmt.Sprintf("%04d-%02d-%02d", year, month, day), month, year
}

func mapRentCycle(value string) string {
	switch strings.TrimSpace(value) {
	case "季付", "季度":
		return "季度"
	case "半年", "半年度":
		return "半年"
	case "年付", "年度", "年度缴纳":
		return "年度"
	default:
		return "月度"
	}
}

func pickFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func getCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func parseFloatValue(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseOptionalNonZeroFloat(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed == 0 {
		return nil
	}
	return &parsed
}

func deleteImportedRowsBySheet(db *sql.DB, sheetName string) error {
	_, err := db.Exec(`DELETE FROM tenants WHERE id LIKE ?`, buildImportIDPrefix(sheetName)+"%")
	return err
}

func writeImportDebugJSON(excelPath string, snapshot importDebugSnapshot) (string, error) {
	baseName := strings.TrimSuffix(filepath.Base(excelPath), filepath.Ext(excelPath))
	outputPath := filepath.Join(filepath.Dir(excelPath), fmt.Sprintf("%s.import.debug.json", baseName))

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(outputPath, payload, 0644); err != nil {
		return "", err
	}
	return outputPath, nil
}
