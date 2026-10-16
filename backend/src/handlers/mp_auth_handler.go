package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"rentadmin/src/middleware"
	"rentadmin/src/models"
	"rentadmin/src/services"
	"rentadmin/src/utils"

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
	openid := c.GetString(middleware.OpenIDKey)

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

	req.TenantName = strings.TrimSpace(req.TenantName)
	if req.TenantName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写租客姓名"})
		return
	}

	// Binding grants access to the room's bills, so the caller must know the current tenant's name.
	var currentName string
	err := h.DB.QueryRow(`SELECT name FROM tenants WHERE room_number = ? ORDER BY date DESC LIMIT 1`, req.RoomNumber).Scan(&currentName)
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err == sql.ErrNoRows || strings.TrimSpace(currentName) != req.TenantName {
		c.JSON(http.StatusForbidden, gin.H{"error": "房号或租客姓名不匹配"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var existingID string
	err = h.DB.QueryRow(`SELECT id FROM mp_user_bindings WHERE openid = ?`, openid).Scan(&existingID)
	if err == nil {
		_, err = h.DB.Exec(`UPDATE mp_user_bindings SET room_number = ?, tenant_name = ?, unionid = ?, updated_at = ? WHERE openid = ?`,
			req.RoomNumber, req.TenantName, nullIfEmpty(req.UnionID), now, openid)
	} else if err == sql.ErrNoRows {
		id := utils.NewID("bind_")
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

// BillView 是小程序账单的对外结构：不含身份证号，手机号脱敏，
// 并补充与上一期相邻账单的对比字段，方便小程序端直接展示本期水电消耗。
type BillView struct {
	ID                     string  `json:"id"`
	RoomNumber             string  `json:"roomNumber"`
	Name                   string  `json:"name"`
	Phone                  string  `json:"phone"`
	RentAmount             float64 `json:"rentAmount"`
	RentDue                float64 `json:"rentDue"`
	ExtraFee               float64 `json:"extraFee"`
	WaterReading           float64 `json:"waterReading"`
	ElectricityReading     float64 `json:"electricityReading"`
	WaterBill              float64 `json:"waterBill"`
	ElectricityBill        float64 `json:"electricityBill"`
	TotalAmount            float64 `json:"totalAmount"`
	AmountPaid             float64 `json:"amountPaid"`
	RentCycle              string  `json:"rentCycle"`
	UtilityCycle           string  `json:"utilityCycle"`
	Status                 string  `json:"status"`
	Date                   string  `json:"date"`
	DueMonth               string  `json:"dueMonth"`
	RecordedAt             *string `json:"recordedAt,omitempty"`
	PrevWaterReading       float64 `json:"prevWaterReading"`
	PrevElectricityReading float64 `json:"prevElectricityReading"`
	WaterUsage             float64 `json:"waterUsage"`
	ElectricityUsage       float64 `json:"electricityUsage"`
	HasPrev                bool    `json:"hasPrev"`
}

func newBillView(t *models.TenantRecord) *BillView {
	return &BillView{
		ID: t.ID, RoomNumber: t.RoomNumber, Name: t.Name, Phone: maskPhone(t.Phone),
		RentAmount: t.RentAmount, RentDue: t.RentDue, ExtraFee: t.ExtraFee, WaterReading: t.WaterReading, ElectricityReading: t.ElectricityReading,
		WaterBill: t.WaterBill, ElectricityBill: t.ElectricityBill, TotalAmount: t.TotalAmount, AmountPaid: t.AmountPaid,
		RentCycle: t.RentCycle, UtilityCycle: t.UtilityCycle, Status: t.Status, Date: t.Date, DueMonth: t.DueMonth, RecordedAt: t.RecordedAt,
	}
}

// maskPhone keeps the first 3 and last 4 digits: 13800138001 -> 138****8001.
func maskPhone(phone string) string {
	if len(phone) < 8 {
		return phone
	}
	return phone[:3] + strings.Repeat("*", len(phone)-7) + phone[len(phone)-4:]
}

func (h *MPAuthHandler) GetBills(c *gin.Context) {
	openid := c.GetString(middleware.OpenIDKey)

	var roomNumber string
	if err := h.DB.QueryRow(`SELECT room_number FROM mp_user_bindings WHERE openid = ?`, openid).Scan(&roomNumber); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "未绑定房号"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list, err := services.QueryTenants(h.DB, `WHERE room_number = ? ORDER BY date DESC`, roomNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录按 date DESC 排列，i+1 即为第 i 条的上一期账单。
	views := make([]*BillView, len(list))
	for i, cur := range list {
		view := newBillView(cur)
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
	openid := c.GetString(middleware.OpenIDKey)

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

func nullIfEmpty(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
