package services

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"rentadmin/src/database"

	"github.com/xuri/excelize/v2"
)

type importDebugFile struct {
	Sheets []importDebugSheet `json:"sheets"`
}

type importDebugSheet struct {
	SheetName  string                `json:"sheetName"`
	Skipped    bool                  `json:"skipped"`
	SkipReason string                `json:"skipReason"`
	ParsedRows []importDebugRowEntry `json:"parsedRows"`
}

type importDebugRowEntry struct {
	Accepted   bool   `json:"accepted"`
	SkipReason string `json:"skipReason"`
}

func TestImportTenantsFromExcel(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "rentadmin.db")
	db := database.OpenDB(dbPath)
	defer db.Close()

	database.InitSchema(db)
	database.MigrateSchema(db)

	workbookPath := filepath.Join(tempDir, "cent.xlsx")
	createWorkbookFixture(t, workbookPath)

	summary, err := ImportTenantsFromExcel(db, workbookPath)
	if err != nil {
		t.Fatalf("first import failed: %v", err)
	}

	if summary.ProcessedSheets != 3 {
		t.Fatalf("expected 3 processed sheets, got %d", summary.ProcessedSheets)
	}
	if summary.Inserted != 4 {
		t.Fatalf("expected 4 inserted rows, got %d", summary.Inserted)
	}
	if summary.Updated != 0 {
		t.Fatalf("expected 0 updated rows on first import, got %d", summary.Updated)
	}

	assertTenantRow(t, db, "101", "2026-01", "Alice", 1000, 0, 1010)
	assertTenantRow(t, db, "101", "2026-04", "Alice", 1000, 500, 1012)
	assertTenantYearFallback(t, db, "201", "2026-01")
	assertTenantRow(t, db, "15", "2026-01", "4楼", 1200, 1220, 1220)
	assertTenantReadings(t, db, "101", "2026-04", 10, 110)

	summary, err = ImportTenantsFromExcel(db, workbookPath)
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}

	if summary.Inserted != 4 {
		t.Fatalf("expected second import to insert 4 rows, got %d", summary.Inserted)
	}
	if summary.Updated != 0 {
		t.Fatalf("expected second import to update 0 rows on sheet overwrite import, got %d", summary.Updated)
	}

	assertCount(t, db, 4)
}

func TestImportTenantsFromExcel_OnlyOverwritesMatchingImportedSheets(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "rentadmin.db")
	db := database.OpenDB(dbPath)
	defer db.Close()

	database.InitSchema(db)
	database.MigrateSchema(db)

	if _, err := db.Exec(`INSERT INTO tenants(id, room_number, name, phone, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"manual-keep", "manual-room", "Manual", "13800000000", 888, 1, 2, 3, 4, 895, 100, "月度", "月度", "部分缴纳", "2026-01", "2026-01-01", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 100, 100, 7, 7, 7); err != nil {
		t.Fatalf("insert manual seed failed: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO tenants(id, room_number, name, phone, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"import-101-202601", "101", "Old Alice", "13800000001", 999, 9, 99, 9, 9, 1017, 0, "月度", "月度", "待缴", "2026-01", "2026-01-01", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 0, 0, 18, 18, 18); err != nil {
		t.Fatalf("insert import seed failed: %v", err)
	}

	workbookPath := filepath.Join(tempDir, "cent.xlsx")
	createWorkbookFixture(t, workbookPath)

	summary, err := ImportTenantsFromExcel(db, workbookPath)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if summary.Inserted != 4 {
		t.Fatalf("expected 4 inserted rows, got %d", summary.Inserted)
	}

	assertCount(t, db, 5)
	assertTenantDateExists(t, db, "manual-room", "2026-01")
	assertTenantRow(t, db, "101", "2026-01", "Alice", 1000, 0, 1010)
}

func TestImportTenantsFromExcel_UsesBottomRowsAsLatestMonths(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "rentadmin.db")
	db := database.OpenDB(dbPath)
	defer db.Close()

	database.InitSchema(db)
	database.MigrateSchema(db)

	workbook := excelize.NewFile()
	workbook.DeleteSheet("Sheet1")
	mustAddSheet(t, workbook, "301", [][]any{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"月份", "电起", "耗电度数", "电费", "水起", "数量", "水费", "额外", "总计金额", "已收", "剩余应收", "房间号", 301, "赵六"},
		{"初始值", 1000, 0, 0, 100, 0, 0, 0, 0, 0, 0, "房租", 1000, "2025-12-01"},
		{1, 1010, 10, 12, 110, 10, 55, 0, 1067, 1067, 0},
		{4, 1030, 20, 24, 120, 10, 55, 0, 1079, 1079, 0},
		{7, 1050, 20, 24, 130, 10, 55, 0, 1079, 1079, 0},
		{10, 1070, 20, 24, 140, 10, 55, 0, 1079, 1079, 0},
		{1, 1090, 20, 24, 150, 10, 55, 0, 1079, 1079, 0},
		{4, 1110, 20, 24, 160, 10, 55, 0, 1079, 1079, 0},
	})

	workbookPath := filepath.Join(tempDir, "cent.xlsx")
	if err := workbook.SaveAs(workbookPath); err != nil {
		t.Fatalf("save workbook: %v", err)
	}

	summary, err := ImportTenantsFromExcel(db, workbookPath)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if summary.Inserted != 6 {
		t.Fatalf("expected 6 inserted rows, got %d", summary.Inserted)
	}

	assertTenantDateExists(t, db, "301", "2026-01")
	assertTenantDateExists(t, db, "301", "2026-04")
	assertTenantDateExists(t, db, "301", "2026-07")
	assertTenantDateExists(t, db, "301", "2026-10")
	assertTenantDateExists(t, db, "301", "2027-01")
	assertTenantDateExists(t, db, "301", "2027-04")
}

func TestImportTenantsFromExcel_SkipsRowsWithZeroElectricityReading(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "rentadmin.db")
	db := database.OpenDB(dbPath)
	defer db.Close()

	database.InitSchema(db)
	database.MigrateSchema(db)

	workbook := excelize.NewFile()
	workbook.DeleteSheet("Sheet1")
	mustAddSheet(t, workbook, "401", [][]any{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"月份", "电起", "耗电度数", "电费", "水起", "数量", "水费", "额外", "总计金额", "已收", "剩余应收", "房间号", 401, "王五"},
		{"初始值", 2000, 0, 0, 200, 0, 0, 0, 0, 0, 0, "房租", 1200, "2025-12-01"},
		{1, 2010, 10, 12, 210, 10, 55, 0, 1267, 1267, 0},
		{4, 0, 0, 0, 0, 0, 0, 0, 1200, 0, 1200},
	})

	workbookPath := filepath.Join(tempDir, "cent.xlsx")
	if err := workbook.SaveAs(workbookPath); err != nil {
		t.Fatalf("save workbook: %v", err)
	}

	summary, err := ImportTenantsFromExcel(db, workbookPath)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if summary.Inserted != 1 {
		t.Fatalf("expected 1 inserted row, got %d", summary.Inserted)
	}

	assertTenantDateExists(t, db, "401", "2026-01")
	assertTenantDateNotExists(t, db, "401", "2026-04")
}

func TestAnalyzeCent3DebugJSON(t *testing.T) {
	debugPath := filepath.Join("..", "..", "..", "cent3.import.debug.json")
	payload, err := os.ReadFile(debugPath)
	if err != nil {
		t.Fatalf("read debug json failed: %v", err)
	}

	var debug importDebugFile
	if err := json.Unmarshal(payload, &debug); err != nil {
		t.Fatalf("unmarshal debug json failed: %v", err)
	}

	skippedSheets := 0
	invalidMonthRows := 0
	zeroElectricityRows := 0
	otherRejectedRows := 0
	acceptedSheets := 0
	acceptedRows := 0

	for _, sheet := range debug.Sheets {
		if sheet.Skipped {
			skippedSheets++
			t.Logf("sheet skipped: %s, reason=%s", sheet.SheetName, sheet.SkipReason)
			continue
		}

		sheetAccepted := 0
		sheetRejected := 0
		for _, row := range sheet.ParsedRows {
			if row.Accepted {
				sheetAccepted++
				acceptedRows++
				continue
			}

			sheetRejected++
			switch row.SkipReason {
			case "invalid month":
				invalidMonthRows++
			case "电起 is empty or zero":
				zeroElectricityRows++
			default:
				otherRejectedRows++
			}
		}

		if sheetAccepted > 0 {
			acceptedSheets++
		}
		if sheetRejected > 0 {
			t.Logf("sheet %s: accepted=%d rejected=%d", sheet.SheetName, sheetAccepted, sheetRejected)
		}
	}

	t.Logf("summary: totalSheets=%d acceptedSheets=%d skippedSheets=%d acceptedRows=%d invalidMonthRows=%d zeroElectricityRows=%d otherRejectedRows=%d",
		len(debug.Sheets), acceptedSheets, skippedSheets, acceptedRows, invalidMonthRows, zeroElectricityRows, otherRejectedRows)

	if acceptedSheets == 0 {
		t.Fatal("expected accepted sheets to be greater than 0")
	}
}

func createWorkbookFixture(t *testing.T, path string) {
	t.Helper()

	workbook := excelize.NewFile()
	workbook.DeleteSheet("Sheet1")

	mustAddSheet(t, workbook, "101", [][]any{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"月份", "电起", "耗电度数", "电费", "水起", "数量", "水费", "额外", "总计金额", "已收", "剩余应收", "房间号", 101, "Alice"},
		{"初始值", 100, 0, 0, 10, 0, 0, 0, 0, 0, 0, "房租", 1000, "", ""},
		{1, 100, 0, 10, 10, 0, 0, 0, 1010, 0, 1010},
		{4, 110, 0, 12, 10, 0, 0, 0, 1012, 500, 512},
	})

	mustAddSheet(t, workbook, "二楼", [][]any{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"月份", "气起", "电起", "耗电度数", "电费", "水起", "数量", "水费", "额外", "总计金额", "已收", "剩余应收", "房间号", 201},
		{"初始值", 0, 200, 0, 1, 50, 0, 1, 0, 0, 0, 0, "电话:", "13900000000"},
		{1, 0, 220, 0, 30, 55, 0, 5, 0, 935, 935, 0},
	})

	mustAddSheet(t, workbook, "17", [][]any{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"月份", "电起", "耗电度数", "电费", "水起", "数量", "水费", "额外", "总计金额", "已收", "剩余应收", "房间号", 15, "4楼"},
		{"初始值", 500, 0, 0, 100, 0, 0, 0, 0, 0, 0, "房租", 1200, "2025、12、1"},
		{1, 600, 0, 20, 100, 0, 0, 0, 1220, 1300, 0},
	})

	mustAddSheet(t, workbook, "总表", [][]any{{"总表"}})
	mustAddSheet(t, workbook, "工作表1", [][]any{{""}})
	mustAddSheet(t, workbook, "101(old)", [][]any{{"月份", "房间号"}})

	if err := workbook.SaveAs(path); err != nil {
		t.Fatalf("save workbook: %v", err)
	}
}

func mustAddSheet(t *testing.T, workbook *excelize.File, name string, rows [][]any) {
	t.Helper()
	workbook.NewSheet(name)
	for rowIndex, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, rowIndex+1)
		if err != nil {
			t.Fatalf("coordinates failed: %v", err)
		}
		if err := workbook.SetSheetRow(name, cell, &row); err != nil {
			t.Fatalf("set sheet row failed: %v", err)
		}
	}
}

func assertTenantRow(t *testing.T, db *sql.DB, room string, date string, name string, rent float64, amountPaid float64, total float64) {
	t.Helper()

	var actualName string
	var actualRent, actualPaid, actualTotal float64
	err := db.QueryRow(`SELECT name, rent_amount, amount_paid, total_amount FROM tenants WHERE room_number = ? AND date = ?`, room, date).
		Scan(&actualName, &actualRent, &actualPaid, &actualTotal)
	if err != nil {
		t.Fatalf("query tenant row failed: %v", err)
	}

	if actualName != name {
		t.Fatalf("expected name %q, got %q", name, actualName)
	}
	if actualRent != rent || actualPaid != amountPaid || actualTotal != total {
		t.Fatalf("unexpected amounts for room %s date %s: rent=%v paid=%v total=%v", room, date, actualRent, actualPaid, actualTotal)
	}
}

func assertTenantYearFallback(t *testing.T, db *sql.DB, room string, expectedDate string) {
	t.Helper()

	var actualDate string
	err := db.QueryRow(`SELECT date FROM tenants WHERE room_number = ?`, room).Scan(&actualDate)
	if err != nil {
		t.Fatalf("query fallback year failed: %v", err)
	}
	if actualDate != expectedDate {
		t.Fatalf("expected fallback date %s, got %s", expectedDate, actualDate)
	}
}

func assertCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()

	var actual int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&actual); err != nil {
		t.Fatalf("count tenants failed: %v", err)
	}
	if actual != expected {
		t.Fatalf("expected %d tenant rows, got %d", expected, actual)
	}
}

func assertTenantReadings(t *testing.T, db *sql.DB, room string, date string, water float64, electricity float64) {
	t.Helper()

	var actualWater, actualElectricity float64
	err := db.QueryRow(`SELECT water_reading, electricity_reading FROM tenants WHERE room_number = ? AND date = ?`, room, date).
		Scan(&actualWater, &actualElectricity)
	if err != nil {
		t.Fatalf("query tenant readings failed: %v", err)
	}

	if actualWater != water || actualElectricity != electricity {
		t.Fatalf("unexpected readings for room %s date %s: water=%v electricity=%v", room, date, actualWater, actualElectricity)
	}
}

func assertTenantDateExists(t *testing.T, db *sql.DB, room string, date string) {
	t.Helper()

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM tenants WHERE room_number = ? AND date = ?`, room, date).Scan(&count)
	if err != nil {
		t.Fatalf("query tenant date failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected room %s date %s to exist once, got %d", room, date, count)
	}
}

func assertTenantDateNotExists(t *testing.T, db *sql.DB, room string, date string) {
	t.Helper()

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM tenants WHERE room_number = ? AND date = ?`, room, date).Scan(&count)
	if err != nil {
		t.Fatalf("query tenant date failed: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected room %s date %s to not exist, got %d", room, date, count)
	}
}
