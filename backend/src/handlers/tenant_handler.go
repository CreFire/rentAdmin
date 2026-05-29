package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"rentadmin/src/models"

	"rentadmin/src/utils"

	"github.com/gin-gonic/gin"
)

// TenantHandler handles tenant-related HTTP requests
type TenantHandler struct {
	DB *sql.DB
}

func getRentCycleMultiplier(cycle string) float64 {
	switch cycle {
	case "月度", "月付", "月":
		return 1
	case "季度":
		return 3
	case "半年", "半年度":
		return 6
	case "年度", "年度缴纳", "年":
		return 12
	default:
		return 1
	}
}

// GetAllTenants handles GET /api/tenants
func (h *TenantHandler) GetAllTenants(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id, room_number, name, phone, id_card, check_in_date, deposit, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income FROM tenants ORDER BY room_number ASC, date DESC`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := make([]*models.TenantRecord, 0)
	for rows.Next() {
		var t models.TenantRecord
		var idCard, checkInDate, recordedAt *string
		var deposit *float64
		var createdAt, updatedAt string
		var monthlyIncome, annualIncome, waterElecIncome, monthlyWaterElecIncome, annualWaterElecIncome float64

		err := rows.Scan(&t.ID, &t.RoomNumber, &t.Name, &t.Phone, &idCard, &checkInDate, &deposit, &t.RentAmount, &t.WaterReading, &t.ElectricityReading, &t.WaterBill, &t.ElectricityBill, &t.TotalAmount, &t.AmountPaid, &t.RentCycle, &t.UtilityCycle, &t.Status, &t.Date, &recordedAt, &createdAt, &updatedAt, &monthlyIncome, &annualIncome, &waterElecIncome, &monthlyWaterElecIncome, &annualWaterElecIncome)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		t.IDCard = idCard
		t.CheckInDate = checkInDate
		t.Deposit = deposit
		t.RecordedAt = recordedAt
		t.MonthlyIncome = monthlyIncome
		t.AnnualIncome = annualIncome
		t.WaterElecIncome = waterElecIncome
		t.MonthlyWaterElecIncome = monthlyWaterElecIncome
		t.AnnualWaterElecIncome = annualWaterElecIncome

		// Parse time strings
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		list = append(list, &t)
	}
	c.JSON(200, list)
}

// GetIncomeSummary handles GET /api/income-summary
func (h *TenantHandler) GetIncomeSummary(c *gin.Context) {
	dateFilter := c.Query("date") // Optional date filter in format "YYYY-MM" or "YYYY"

	summary, err := utils.GetIncomeSummary(h.DB, dateFilter)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"totalReceivable":    summary.TotalAmount,     // 累计应收
		"totalReceived":      summary.AmountPaid,      // 累计实收
		"totalUtilityIncome": summary.WaterElecIncome, // 水电总收入
		"outstandingBalance": summary.MonthlyIncome,   // 代收余额（未收金额）
		"dateFilter":         dateFilter,
	})
}

// CreateOrUpdateTenant handles POST /api/tenants
func (h *TenantHandler) CreateOrUpdateTenant(c *gin.Context) {
	var req models.TenantRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Calculate bills if they're not provided
	calculatedWaterBill := req.WaterBill
	calculatedElectricityBill := req.ElectricityBill
	rentCycleMultiplier := getRentCycleMultiplier(req.RentCycle)
	calculatedRentAmount := req.RentAmount * rentCycleMultiplier
	calculatedTotalAmount := calculatedRentAmount + calculatedWaterBill + calculatedElectricityBill

	// If bills are not provided, calculate them based on readings
	if req.WaterBill == 0 && req.ElectricityBill == 0 {
		// Get previous record to calculate usage
		var prevWaterReading, prevElectricityReading float64
		var prevDate string
		err := h.DB.QueryRow("SELECT water_reading, electricity_reading, date FROM tenants WHERE room_number = ? AND date < ? ORDER BY date DESC LIMIT 1",
			req.RoomNumber, req.Date).Scan(&prevWaterReading, &prevElectricityReading, &prevDate)

		if err == nil {
			// Calculate usage from previous reading
			waterUsage := req.WaterReading - prevWaterReading
			elecUsage := req.ElectricityReading - prevElectricityReading
			calculatedWaterBill = waterUsage * models.WATER_UNIT_PRICE
			calculatedElectricityBill = elecUsage * models.ELEC_UNIT_PRICE
		} else {
			// No previous record, use provided readings as is
			calculatedWaterBill = 0
			calculatedElectricityBill = 0
		}

		calculatedTotalAmount = calculatedRentAmount + calculatedWaterBill + calculatedElectricityBill
	}

	// Determine status based on payment
	status := req.Status
	// Ensure calculatedTotalAmount is never negative
	if calculatedTotalAmount < 0 {
		calculatedTotalAmount = 0
	}
	// Ensure AmountPaid is never negative
	amountPaid := req.AmountPaid
	if amountPaid < 0 {
		amountPaid = 0
	}

	// Fix: Prevent AmountPaid from exceeding TotalAmount to avoid negative代收余额
	if amountPaid > calculatedTotalAmount {
		amountPaid = calculatedTotalAmount
	}

	if amountPaid >= calculatedTotalAmount {
		status = "已缴"
	} else if amountPaid > 0 {
		status = "部分缴纳"
	} else {
		status = "待缴"
	}

	// Calculate income statistics
	monthlyIncome := amountPaid
	annualIncome := amountPaid
	waterElecIncome := calculatedWaterBill + calculatedElectricityBill
	monthlyWaterElecIncome := waterElecIncome
	annualWaterElecIncome := waterElecIncome

	// Check if record already exists
	var existingID string
	err := h.DB.QueryRow("SELECT id FROM tenants WHERE room_number = ? AND date = ?", req.RoomNumber, req.Date).Scan(&existingID)

	if err == nil {
		// Record exists, update it
		_, err = h.DB.Exec(`UPDATE tenants SET name=?, phone=?, id_card=?, check_in_date=?, deposit=?, rent_amount=?, water_reading=?, electricity_reading=?, water_bill=?, electricity_bill=?, total_amount=?, amount_paid=?, rent_cycle=?, utility_cycle=?, status=?, recorded_at=?, updated_at=?, monthly_income=?, annual_income=?, water_elec_income=?, monthly_water_elec_income=?, annual_water_elec_income=? WHERE room_number=? AND date=?`,
			req.Name, req.Phone, req.IDCard, req.CheckInDate, req.Deposit, req.RentAmount, req.WaterReading, req.ElectricityReading, calculatedWaterBill, calculatedElectricityBill, calculatedTotalAmount, amountPaid, req.RentCycle, req.UtilityCycle, status, req.RecordedAt, time.Now().UTC(), monthlyIncome, annualIncome, waterElecIncome, monthlyWaterElecIncome, annualWaterElecIncome, req.RoomNumber, req.Date)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"id": existingID, "message": "Tenant record updated successfully"})
	} else {
		// Record doesn't exist, create new one
		id := req.ID
		if id == "" {
			id = time.Now().Format("20060102150405") // Generate ID from timestamp
		}

		_, err := h.DB.Exec(`INSERT INTO tenants(id, room_number, name, phone, id_card, check_in_date, deposit, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, req.RoomNumber, req.Name, req.Phone, req.IDCard, req.CheckInDate, req.Deposit, req.RentAmount, req.WaterReading, req.ElectricityReading, calculatedWaterBill, calculatedElectricityBill, calculatedTotalAmount, amountPaid, req.RentCycle, req.UtilityCycle, status, req.Date, req.RecordedAt, time.Now().UTC(), time.Now().UTC(), monthlyIncome, annualIncome, waterElecIncome, monthlyWaterElecIncome, annualWaterElecIncome)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"id": id})
	}
}

// UpdateTenant handles PUT /api/tenants/:id
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	id := c.Param("id")
	log.Printf("Received PUT request for tenant ID: %s", id)

	var req models.TenantRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Calculate bills if they're not provided
	calculatedWaterBill := req.WaterBill
	calculatedElectricityBill := req.ElectricityBill
	rentCycleMultiplier := getRentCycleMultiplier(req.RentCycle)
	calculatedRentAmount := req.RentAmount * rentCycleMultiplier
	calculatedTotalAmount := calculatedRentAmount + calculatedWaterBill + calculatedElectricityBill

	// If bills are not provided, calculate them based on readings
	if req.WaterBill == 0 && req.ElectricityBill == 0 {
		// Get previous record to calculate usage
		var prevWaterReading, prevElectricityReading float64
		var prevDate string
		err := h.DB.QueryRow("SELECT water_reading, electricity_reading, date FROM tenants WHERE room_number = ? AND date < ? ORDER BY date DESC LIMIT 1",
			req.RoomNumber, req.Date).Scan(&prevWaterReading, &prevElectricityReading, &prevDate)

		if err == nil {
			// Calculate usage from previous reading
			waterUsage := req.WaterReading - prevWaterReading
			elecUsage := req.ElectricityReading - prevElectricityReading
			calculatedWaterBill = waterUsage * models.WATER_UNIT_PRICE
			calculatedElectricityBill = elecUsage * models.ELEC_UNIT_PRICE
		} else {
			// No previous record, use provided readings as is
			calculatedWaterBill = 0
			calculatedElectricityBill = 0
		}

		calculatedTotalAmount = calculatedRentAmount + calculatedWaterBill + calculatedElectricityBill
	}

	// Determine status based on payment
	status := req.Status
	// Ensure calculatedTotalAmount is never negative
	if calculatedTotalAmount < 0 {
		calculatedTotalAmount = 0
	}
	// Ensure AmountPaid is never negative
	amountPaid := req.AmountPaid
	if amountPaid < 0 {
		amountPaid = 0
	}

	// Fix: Prevent AmountPaid from exceeding TotalAmount to avoid negative代收余额
	if amountPaid > calculatedTotalAmount {
		amountPaid = calculatedTotalAmount
	}

	if amountPaid >= calculatedTotalAmount {
		status = "已缴"
	} else if amountPaid > 0 {
		status = "部分缴纳"
	} else {
		status = "待缴"
	}

	// Calculate income statistics
	monthlyIncome := amountPaid
	annualIncome := amountPaid
	waterElecIncome := calculatedWaterBill + calculatedElectricityBill
	monthlyWaterElecIncome := waterElecIncome
	annualWaterElecIncome := waterElecIncome

	result, err := h.DB.Exec(`UPDATE tenants SET name=?, phone=?, id_card=?, check_in_date=?, deposit=?, rent_amount=?, water_reading=?, electricity_reading=?, water_bill=?, electricity_bill=?, total_amount=?, amount_paid=?, rent_cycle=?, utility_cycle=?, status=?, recorded_at=?, updated_at=?, monthly_income=?, annual_income=?, water_elec_income=?, monthly_water_elec_income=?, annual_water_elec_income=? WHERE id = ?`,
		req.Name, req.Phone, req.IDCard, req.CheckInDate, req.Deposit, req.RentAmount, req.WaterReading, req.ElectricityReading, calculatedWaterBill, calculatedElectricityBill, calculatedTotalAmount, amountPaid, req.RentCycle, req.UtilityCycle, status, req.RecordedAt, time.Now().UTC(), monthlyIncome, annualIncome, waterElecIncome, monthlyWaterElecIncome, annualWaterElecIncome, id)
	if err != nil {
		log.Printf("Error executing update: %v", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Rows affected: %d", rowsAffected)
	if rowsAffected == 0 {
		log.Printf("No tenant found with ID: %s", id)
		c.JSON(404, gin.H{"error": "Tenant not found"})
	} else {
		log.Printf("Tenant record updated successfully: %s", id)
		c.JSON(200, gin.H{"id": id, "message": "Tenant record updated successfully"})
	}
}

// GetTenantsByRoom handles GET /api/tenants/room/:room_number
func (h *TenantHandler) GetTenantsByRoom(c *gin.Context) {
	roomNumber := c.Param("room_number")

	rows, err := h.DB.Query(`SELECT id, room_number, name, phone, id_card, check_in_date, deposit, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income FROM tenants WHERE room_number = ? ORDER BY date DESC`, roomNumber)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := make([]*models.TenantRecord, 0)
	for rows.Next() {
		var t models.TenantRecord
		var idCard, checkInDate, recordedAt *string
		var deposit *float64
		var createdAt, updatedAt string
		var monthlyIncome, annualIncome, waterElecIncome, monthlyWaterElecIncome, annualWaterElecIncome float64

		err := rows.Scan(&t.ID, &t.RoomNumber, &t.Name, &t.Phone, &idCard, &checkInDate, &deposit, &t.RentAmount, &t.WaterReading, &t.ElectricityReading, &t.WaterBill, &t.ElectricityBill, &t.TotalAmount, &t.AmountPaid, &t.RentCycle, &t.UtilityCycle, &t.Status, &t.Date, &recordedAt, &createdAt, &updatedAt, &monthlyIncome, &annualIncome, &waterElecIncome, &monthlyWaterElecIncome, &annualWaterElecIncome)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		t.IDCard = idCard
		t.CheckInDate = checkInDate
		t.Deposit = deposit
		t.RecordedAt = recordedAt
		t.MonthlyIncome = monthlyIncome
		t.AnnualIncome = annualIncome
		t.WaterElecIncome = waterElecIncome
		t.MonthlyWaterElecIncome = monthlyWaterElecIncome
		t.AnnualWaterElecIncome = annualWaterElecIncome

		// Parse time strings
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		list = append(list, &t)
	}
	c.JSON(200, list)
}

// DeleteTenantByRoom handles DELETE /api/tenants/room/:room_number
func (h *TenantHandler) DeleteTenantByRoom(c *gin.Context) {
	roomNumber := c.Param("room_number")
	if roomNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room number is required"})
		return
	}

	result, err := h.DB.Exec("DELETE FROM tenants WHERE room_number = ?", roomNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	deletedRows, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if deletedRows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Tenant deleted successfully",
		"deletedRows": deletedRows,
		"roomNumber":  roomNumber,
	})
}

// DeleteTenantByID handles DELETE /api/tenants/:id
func (h *TenantHandler) DeleteTenantByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	result, err := h.DB.Exec("DELETE FROM tenants WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	deletedRows, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if deletedRows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Tenant record deleted successfully",
		"deletedRows": deletedRows,
		"id":          id,
	})
}

// ClearTenants handles DELETE /api/tenants
func (h *TenantHandler) ClearTenants(c *gin.Context) {
	result, err := h.DB.Exec("DELETE FROM tenants")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	deletedRows, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "All tenant records cleared successfully",
		"deletedRows": deletedRows,
	})
}
