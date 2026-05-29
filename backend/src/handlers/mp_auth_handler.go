package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"rentadmin/src/models"
	"rentadmin/src/services"

	"github.com/gin-gonic/gin"
)

type MPAuthHandler struct {
	DB       *sql.DB
	Auth     *services.AuthService
	WeChat   *services.WeChatPayService
	Reminder *services.ReminderService
}

func (h *MPAuthHandler) Login(c *gin.Context) {
	var req models.MPLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	openid, unionid, err := h.WeChat.Login(req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, expiry, err := h.Auth.GenerateToken(openid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresAt": expiry,
		"openid":    openid,
		"unionId":   unionid,
	})
}

func (h *MPAuthHandler) BindRoom(c *gin.Context) {
	openid, ok := h.requireOpenID(c)
	if !ok {
		return
	}

	var req models.MPBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.RoomNumber = strings.TrimSpace(req.RoomNumber)
	if req.RoomNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "roomNumber is required"})
		return
	}

	var tenantCount int
	err := h.DB.QueryRow(`SELECT COUNT(1) FROM tenants WHERE room_number = ?`, req.RoomNumber).Scan(&tenantCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tenantCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var existingID string
	err = h.DB.QueryRow(`SELECT id FROM mp_user_bindings WHERE openid = ?`, openid).Scan(&existingID)
	if err == nil {
		_, err = h.DB.Exec(`UPDATE mp_user_bindings SET room_number = ?, tenant_name = ?, unionid = ?, updated_at = ? WHERE openid = ?`,
			req.RoomNumber, req.TenantName, nullIfEmpty(req.UnionID), now, openid)
	} else if err == sql.ErrNoRows {
		id := "bind_" + time.Now().UTC().Format("20060102150405.000000000")
		_, err = h.DB.Exec(`INSERT INTO mp_user_bindings(id, openid, unionid, room_number, tenant_name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, openid, nullIfEmpty(req.UnionID), req.RoomNumber, req.TenantName, now, now)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "绑定成功", "roomNumber": req.RoomNumber})
}

// BillView 在 TenantRecord 基础上补充与上一期相邻账单的对比字段，
// 方便小程序端直接展示本期水电消耗。
type BillView struct {
	*models.TenantRecord
	PrevWaterReading       float64 `json:"prevWaterReading"`
	PrevElectricityReading float64 `json:"prevElectricityReading"`
	WaterUsage             float64 `json:"waterUsage"`
	ElectricityUsage       float64 `json:"electricityUsage"`
	HasPrev                bool    `json:"hasPrev"`
}

func (h *MPAuthHandler) GetBills(c *gin.Context) {
	openid, ok := h.requireOpenID(c)
	if !ok {
		return
	}

	var roomNumber string
	if err := h.DB.QueryRow(`SELECT room_number FROM mp_user_bindings WHERE openid = ?`, openid).Scan(&roomNumber); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "未绑定房号"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, err := h.DB.Query(`SELECT id, room_number, name, phone, id_card, check_in_date, deposit, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income FROM tenants WHERE room_number = ? ORDER BY date DESC`, roomNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		scanErr := rows.Scan(&t.ID, &t.RoomNumber, &t.Name, &t.Phone, &idCard, &checkInDate, &deposit, &t.RentAmount, &t.WaterReading, &t.ElectricityReading, &t.WaterBill, &t.ElectricityBill, &t.TotalAmount, &t.AmountPaid, &t.RentCycle, &t.UtilityCycle, &t.Status, &t.Date, &recordedAt, &createdAt, &updatedAt, &monthlyIncome, &annualIncome, &waterElecIncome, &monthlyWaterElecIncome, &annualWaterElecIncome)
		if scanErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": scanErr.Error()})
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
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		list = append(list, &t)
	}

	// 记录按 date DESC 排列，i+1 即为第 i 条的上一期账单。
	views := make([]*BillView, len(list))
	for i, cur := range list {
		view := &BillView{TenantRecord: cur}
		if i+1 < len(list) {
			prev := list[i+1]
			view.PrevWaterReading = prev.WaterReading
			view.PrevElectricityReading = prev.ElectricityReading
			view.WaterUsage = cur.WaterReading - prev.WaterReading
			view.ElectricityUsage = cur.ElectricityReading - prev.ElectricityReading
			view.HasPrev = true
		}
		views[i] = view
	}

	c.JSON(http.StatusOK, gin.H{"roomNumber": roomNumber, "records": views})
}

func (h *MPAuthHandler) RecordSubscribe(c *gin.Context) {
	openid, ok := h.requireOpenID(c)
	if !ok {
		return
	}

	var req models.MPSubscribeRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Reminder.RecordSubscription(openid, req.TemplateID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "订阅记录成功"})
}

func (h *MPAuthHandler) requireOpenID(c *gin.Context) (string, bool) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization"})
		return "", false
	}

	openid, err := h.Auth.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return "", false
	}
	return openid, true
}

func nullIfEmpty(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
