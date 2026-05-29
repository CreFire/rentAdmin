package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rentadmin/src/models"
	"rentadmin/src/services"

	"github.com/gin-gonic/gin"
)

type WXPayHandler struct {
	DB            *sql.DB
	Auth          *services.AuthService
	WeChat        *services.WeChatPayService
	TenantPayment *services.TenantPaymentService
	Reminder      *services.ReminderService
}

func (h *WXPayHandler) CreateOrder(c *gin.Context) {
	openid, ok := h.requireOpenID(c)
	if !ok {
		return
	}

	var req models.MPCreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TenantBillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantBillId is required"})
		return
	}

	var boundRoom string
	if err := h.DB.QueryRow(`SELECT room_number FROM mp_user_bindings WHERE openid = ?`, openid).Scan(&boundRoom); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "请先绑定房号"})
		return
	}

	var roomNumber, billDate, tenantName string
	var totalAmount, amountPaid float64
	err := h.DB.QueryRow(`SELECT room_number, date, name, total_amount, amount_paid FROM tenants WHERE id = ?`,
		req.TenantBillID).Scan(&roomNumber, &billDate, &tenantName, &totalAmount, &amountPaid)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if roomNumber != boundRoom {
		c.JSON(http.StatusForbidden, gin.H{"error": "账单不属于当前绑定房号"})
		return
	}

	outstandingFen := int64((totalAmount - amountPaid) * 100)
	if outstandingFen <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账单已结清"})
		return
	}

	payAmountFen := outstandingFen
	if req.AmountFen > 0 && req.AmountFen < outstandingFen {
		payAmountFen = req.AmountFen
	}

	now := time.Now().UTC()
	outTradeNo := "ra" + now.Format("20060102150405") + strings.ReplaceAll(time.Now().Format("000000000"), ".", "")
	orderID := "ord_" + now.Format("20060102150405.000000000")
	description := req.Description
	if description == "" {
		description = tenantName + " " + roomNumber + " " + billDate + " 租金支付"
	}

	_, err = h.DB.Exec(`INSERT INTO wx_payment_orders(
id, out_trade_no, tenant_bill_id, room_number, bill_date, openid, amount_fen, currency, description, trade_state, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, 'CNY', ?, 'CREATED', ?, ?)`,
		orderID, outTradeNo, req.TenantBillID, roomNumber, billDate, openid, payAmountFen, description,
		now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	createResult, err := h.WeChat.CreateOrder(services.CreateOrderInput{
		OutTradeNo:  outTradeNo,
		OpenID:      openid,
		AmountFen:   payAmountFen,
		Description: description,
		Attach:      req.TenantBillID,
		ClientIP:    c.ClientIP(),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, _ = h.DB.Exec(`UPDATE wx_payment_orders SET prepay_id = ?, updated_at = ? WHERE out_trade_no = ?`,
		createResult.PrepayID, time.Now().UTC().Format(time.RFC3339), outTradeNo)

	c.JSON(http.StatusOK, gin.H{
		"outTradeNo": outTradeNo,
		"amountFen":  payAmountFen,
		"payParams":  createResult.PayParams,
	})
}

func (h *WXPayHandler) Notify(c *gin.Context) {
	var req models.MPNotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OutTradeNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outTradeNo is required"})
		return
	}

	now := time.Now().UTC()
	paidAt := now
	if req.PaidAt != "" {
		if t, parseErr := time.Parse(time.RFC3339, req.PaidAt); parseErr == nil {
			paidAt = t
		}
	}

	logID := "nlog_" + now.Format("20060102150405.000000000")
	verifyOK := 1
	processed := 0
	if req.Success {
		processed = 1
	}
	_, _ = h.DB.Exec(`INSERT INTO wx_pay_notify_logs(id, out_trade_no, wx_event_id, verify_ok, processed, request_body, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		logID, req.OutTradeNo, nullIfEmpty(req.EventID), verifyOK, processed, toJSONString(req), now.Format(time.RFC3339))

	if !req.Success {
		_, _ = h.DB.Exec(`UPDATE wx_payment_orders SET trade_state = 'PAYERROR', updated_at = ?, sync_err = ? WHERE out_trade_no = ?`,
			now.Format(time.RFC3339), "notify success=false", req.OutTradeNo)
		c.JSON(http.StatusOK, gin.H{"message": "notify received but marked as failed"})
		return
	}

	if err := h.TenantPayment.ApplyPaymentByOutTradeNo(req.OutTradeNo, req.TransactionID, paidAt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, _ = h.DB.Exec(`UPDATE wx_pay_notify_logs SET processed = 1 WHERE id = ?`, logID)
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *WXPayHandler) GetOrder(c *gin.Context) {
	openid, ok := h.requireOpenID(c)
	if !ok {
		return
	}
	outTradeNo := c.Param("out_trade_no")
	if outTradeNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "out_trade_no is required"})
		return
	}

	var order models.WeChatPaymentOrder
	var wxTxID, prepayID, paidAt sql.NullString
	var createdAt, updatedAt string
	var notifyProcessed int
	err := h.DB.QueryRow(`SELECT id, out_trade_no, tenant_bill_id, room_number, bill_date, openid, amount_fen, currency, description, trade_state, wx_transaction_id, prepay_id, notify_processed, paid_at, created_at, updated_at
FROM wx_payment_orders WHERE out_trade_no = ?`, outTradeNo).Scan(
		&order.ID, &order.OutTradeNo, &order.TenantBillID, &order.RoomNumber, &order.BillDate, &order.OpenID, &order.AmountFen, &order.Currency, &order.Description, &order.TradeState,
		&wxTxID, &prepayID, &notifyProcessed, &paidAt, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if order.OpenID != openid {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	order.NotifyProcessed = notifyProcessed == 1
	if wxTxID.Valid {
		order.WXTransactionID = &wxTxID.String
	}
	if prepayID.Valid {
		order.PrepayID = &prepayID.String
	}
	if paidAt.Valid {
		if t, parseErr := time.Parse(time.RFC3339, paidAt.String); parseErr == nil {
			order.PaidAt = &t
		}
	}
	order.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	order.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	c.JSON(http.StatusOK, order)
}

func (h *WXPayHandler) SyncOrder(c *gin.Context) {
	openid, ok := h.requireOpenID(c)
	if !ok {
		return
	}
	outTradeNo := c.Param("out_trade_no")
	if outTradeNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "out_trade_no is required"})
		return
	}

	var orderOpenID string
	err := h.DB.QueryRow(`SELECT openid FROM wx_payment_orders WHERE out_trade_no = ?`, outTradeNo).Scan(&orderOpenID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if orderOpenID != openid {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	state, transactionID, err := h.WeChat.QueryOrder(outTradeNo)
	now := time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		_, _ = h.DB.Exec(`UPDATE wx_payment_orders SET last_sync_at = ?, sync_err = ?, updated_at = ? WHERE out_trade_no = ?`,
			now, err.Error(), now, outTradeNo)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, _ = h.DB.Exec(`UPDATE wx_payment_orders SET last_sync_at = ?, sync_err = '', updated_at = ? WHERE out_trade_no = ?`,
		now, now, outTradeNo)
	if state == "SUCCESS" {
		if applyErr := h.TenantPayment.ApplyPaymentByOutTradeNo(outTradeNo, transactionID, time.Now().UTC()); applyErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": applyErr.Error(), "tradeState": state})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"outTradeNo": outTradeNo, "tradeState": state})
}

func (h *WXPayHandler) TriggerReminders(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	sent1, fail1, err := h.Reminder.RunDueReminders(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sent2, fail2, err := h.Reminder.RetryFailedReminders(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sent":           sent1 + sent2,
		"failed":         fail1 + fail2,
		"processedLimit": limit,
	})
}

func (h *WXPayHandler) requireOpenID(c *gin.Context) (string, bool) {
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

func toJSONString(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
