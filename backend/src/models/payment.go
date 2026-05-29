package models

import "time"

// WeChatPaymentOrder represents a payment order mapped to wx_payment_orders table.
type WeChatPaymentOrder struct {
	ID              string     `json:"id"`
	OutTradeNo      string     `json:"outTradeNo"`
	TenantBillID    string     `json:"tenantBillId"`
	RoomNumber      string     `json:"roomNumber"`
	BillDate        string     `json:"billDate"`
	OpenID          string     `json:"openid"`
	AmountFen       int64      `json:"amountFen"`
	Currency        string     `json:"currency"`
	Description     string     `json:"description"`
	TradeState      string     `json:"tradeState"`
	WXTransactionID *string    `json:"wxTransactionId,omitempty"`
	PrepayID        *string    `json:"prepayId,omitempty"`
	NotifyProcessed bool       `json:"notifyProcessed"`
	PaidAt          *time.Time `json:"paidAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	LastSyncAt      *time.Time `json:"lastSyncAt,omitempty"`
	SyncErr         *string    `json:"syncErr,omitempty"`
}

type MPLoginRequest struct {
	Code string `json:"code"`
}

type MPBindRequest struct {
	RoomNumber string `json:"roomNumber"`
	TenantName string `json:"tenantName"`
	UnionID    string `json:"unionId"`
}

type MPCreateOrderRequest struct {
	TenantBillID string `json:"tenantBillId"`
	AmountFen    int64  `json:"amountFen"`
	Description  string `json:"description"`
}

type MPSubscribeRecordRequest struct {
	TemplateID string `json:"templateId"`
}

type MPNotifyRequest struct {
	OutTradeNo    string `json:"outTradeNo"`
	TransactionID string `json:"transactionId"`
	Success       bool   `json:"success"`
	EventID       string `json:"eventId"`
	PaidAt        string `json:"paidAt"`
}
