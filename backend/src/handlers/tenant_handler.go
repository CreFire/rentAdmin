package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"rentadmin/src/models"
	"rentadmin/src/services"
	"rentadmin/src/utils"

	"github.com/gin-gonic/gin"
)

var billDatePattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// TenantHandler handles tenant-related HTTP requests
type TenantHandler struct {
	DB *sql.DB
}

// GetAllTenants handles GET /api/tenants
func (h *TenantHandler) GetAllTenants(c *gin.Context) {
	list, err := services.QueryTenants(h.DB, `ORDER BY room_number ASC, date DESC`)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// GetIncomeSummary handles GET /api/income-summary
func (h *TenantHandler) GetIncomeSummary(c *gin.Context) {
	dateFilter := c.Query("date") // Optional date filter in format "YYYY-MM" or "YYYY"

	summary, err := utils.GetIncomeSummary(h.DB, dateFilter)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"totalReceivable":    summary.TotalAmount,     // 累计应收
		"totalReceived":      summary.AmountPaid,      // 累计实收
		"totalUtilityIncome": summary.WaterElecIncome, // 水电总收入
		"outstandingBalance": summary.MonthlyIncome,   // 代收余额（未收金额）
		"dateFilter":         dateFilter,
	})
}

// CreateOrUpdateTenant handles POST /api/tenants.
// A bill is identified by (roomNumber, date): an existing one is overwritten, otherwise a new one is inserted.
func (h *TenantHandler) CreateOrUpdateTenant(c *gin.Context) {
	var req models.TenantRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RoomNumber == "" || req.Name == "" || !billDatePattern.MatchString(req.Date) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "roomNumber、name 必填，date 格式为 YYYY-MM"})
		return
	}
	if err := services.CalculateBill(h.DB, &req, true); err != nil {
		respondError(c, err)
		return
	}
	req.UpdatedAt = time.Now().UTC()

	var existingID string
	err := h.DB.QueryRow(`SELECT id FROM tenants WHERE room_number = ? AND date = ?`, req.RoomNumber, req.Date).Scan(&existingID)
	switch {
	case err == nil:
		if _, err := services.UpdateTenant(h.DB, existingID, &req); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": existingID, "message": "Tenant record updated successfully"})
	case errors.Is(err, sql.ErrNoRows):
		if req.ID == "" {
			req.ID = utils.NewID("t_")
		}
		req.CreatedAt = req.UpdatedAt
		if err := services.InsertTenant(h.DB, &req); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": req.ID})
	default:
		respondError(c, err)
	}
}

// UpdateTenant handles PUT /api/tenants/:id.
// The request body is merged onto the stored record, so omitted fields keep their current values.
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	id := c.Param("id")
	record, err := services.GetTenantByID(h.DB, id)
	if err != nil {
		respondError(c, err)
		return
	}
	roomNumber, date := record.RoomNumber, record.Date
	rentKeyBefore := rentKey(record)

	if err := c.ShouldBindJSON(record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Bill identity is immutable through this endpoint.
	record.ID, record.RoomNumber, record.Date = id, roomNumber, date

	// Rent is only re-derived when its inputs change, so totals of imported bills survive contact edits.
	if err := services.CalculateBill(h.DB, record, rentKey(record) != rentKeyBefore); err != nil {
		respondError(c, err)
		return
	}
	record.UpdatedAt = time.Now().UTC()
	if _, err := services.UpdateTenant(h.DB, id, record); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "message": "Tenant record updated successfully"})
}

// rentKey captures the inputs of the rent rule (monthly rent, cycle, move-in date).
func rentKey(t *models.TenantRecord) string {
	checkIn := ""
	if t.CheckInDate != nil {
		checkIn = *t.CheckInDate
	}
	return fmt.Sprintf("%.2f|%s|%s", t.RentAmount, t.RentCycle, checkIn)
}

type generateBillsRequest struct {
	Month string `json:"month"`
}

// GenerateMonthlyBills handles POST /api/bills/generate {"month":"YYYY-MM"} (defaults to the current month).
func (h *TenantHandler) GenerateMonthlyBills(c *gin.Context) {
	var req generateBillsRequest
	_ = c.ShouldBindJSON(&req)
	if req.Month == "" {
		req.Month = time.Now().Format("2006-01")
	}
	if !billDatePattern.MatchString(req.Month) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "month 格式为 YYYY-MM"})
		return
	}
	result, err := services.GenerateMonthlyBills(h.DB, req.Month)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type registerPaymentRequest struct {
	Amount float64 `json:"amount"`
}

// RegisterPayment handles POST /api/tenants/:id/payments — adds a received amount to the bill.
func (h *TenantHandler) RegisterPayment(c *gin.Context) {
	var req registerPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount 必须为正数"})
		return
	}
	record, err := services.RegisterPayment(h.DB, c.Param("id"), req.Amount)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, record)
}

// RegisterRoomPayment handles POST /api/rooms/:room_number/payments — one collection for a room,
// applied to its unpaid bills from the oldest due month on.
func (h *TenantHandler) RegisterRoomPayment(c *gin.Context) {
	var req registerPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount 必须为正数"})
		return
	}
	result, err := services.RegisterRoomPayment(h.DB, c.Param("room_number"), req.Amount)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetTenantsByRoom handles GET /api/tenants/room/:room_number
func (h *TenantHandler) GetTenantsByRoom(c *gin.Context) {
	list, err := services.QueryTenants(h.DB, `WHERE room_number = ? ORDER BY date DESC`, c.Param("room_number"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// DeleteTenantByRoom handles DELETE /api/tenants/room/:room_number
func (h *TenantHandler) DeleteTenantByRoom(c *gin.Context) {
	roomNumber := c.Param("room_number")
	deletedRows, ok := h.deleteWhere(c, "DELETE FROM tenants WHERE room_number = ?", roomNumber)
	if !ok {
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
	deletedRows, ok := h.deleteWhere(c, "DELETE FROM tenants WHERE id = ?", id)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":     "Tenant record deleted successfully",
		"deletedRows": deletedRows,
		"id":          id,
	})
}

func (h *TenantHandler) deleteWhere(c *gin.Context, query string, arg string) (int64, bool) {
	if arg == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifier is required"})
		return 0, false
	}
	result, err := h.DB.Exec(query, arg)
	if err != nil {
		respondError(c, err)
		return 0, false
	}
	deletedRows, err := result.RowsAffected()
	if err != nil {
		respondError(c, err)
		return 0, false
	}
	if deletedRows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return 0, false
	}
	return deletedRows, true
}

// ClearTenants handles DELETE /api/tenants?confirm=ALL
func (h *TenantHandler) ClearTenants(c *gin.Context) {
	if c.Query("confirm") != "ALL" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "清空全部记录需要参数 confirm=ALL"})
		return
	}
	result, err := h.DB.Exec("DELETE FROM tenants")
	if err != nil {
		respondError(c, err)
		return
	}
	deletedRows, _ := result.RowsAffected()
	log.Printf("all tenant records cleared from %s, deletedRows=%d", c.ClientIP(), deletedRows)
	c.JSON(http.StatusOK, gin.H{
		"message":     "All tenant records cleared successfully",
		"deletedRows": deletedRows,
	})
}

// respondError maps known service errors to HTTP status codes.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrTenantNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
	case errors.Is(err, services.ErrNegativeUsage):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("request %s %s failed: %v", c.Request.Method, c.FullPath(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
