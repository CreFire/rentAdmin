package services

import (
	"path/filepath"
	"testing"
	"time"

	"rentadmin/src/database"
)

func TestApplyPaymentByOutTradeNo_PartialCapAndIdempotent(t *testing.T) {
	tempDir := t.TempDir()
	db := database.OpenDB(filepath.Join(tempDir, "payment.db"))
	defer db.Close()
	database.InitSchema(db)
	database.MigrateSchema(db)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO tenants(
id, room_number, name, phone, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bill-1", "101", "Alice", "13800000000", 1000, 0, 0, 0, 0, 1000, 100, "月度", "月度", "部分缴纳", "2026-05", "2026-05-01", now, now, 100, 100, 0, 0, 0)
	if err != nil {
		t.Fatalf("seed tenant failed: %v", err)
	}

	_, err = db.Exec(`INSERT INTO wx_payment_orders(
id, out_trade_no, tenant_bill_id, room_number, bill_date, openid, amount_fen, currency, description, trade_state, created_at, updated_at
) VALUES(?, ?, ?, ?, ?, ?, ?, 'CNY', ?, 'CREATED', ?, ?)`,
		"order-1", "otn-1", "bill-1", "101", "2026-05", "mock_openid", 5000, "partial payment", now, now)
	if err != nil {
		t.Fatalf("seed order failed: %v", err)
	}

	service := NewTenantPaymentService(db)
	if err = service.ApplyPaymentByOutTradeNo("otn-1", "wx_tx_1", time.Now().UTC()); err != nil {
		t.Fatalf("apply partial payment failed: %v", err)
	}

	var amountPaid float64
	var status string
	if err = db.QueryRow(`SELECT amount_paid, status FROM tenants WHERE id = ?`, "bill-1").Scan(&amountPaid, &status); err != nil {
		t.Fatalf("query tenant after payment failed: %v", err)
	}
	if amountPaid != 150 {
		t.Fatalf("expected amount_paid=150 got %.2f", amountPaid)
	}
	if status != "部分缴纳" {
		t.Fatalf("expected status=部分缴纳 got %s", status)
	}

	// same notify again should be idempotent
	if err = service.ApplyPaymentByOutTradeNo("otn-1", "wx_tx_1", time.Now().UTC()); err != nil {
		t.Fatalf("second apply should be idempotent, got %v", err)
	}
	if err = db.QueryRow(`SELECT amount_paid FROM tenants WHERE id = ?`, "bill-1").Scan(&amountPaid); err != nil {
		t.Fatalf("query tenant after idempotent apply failed: %v", err)
	}
	if amountPaid != 150 {
		t.Fatalf("idempotent apply changed amount_paid unexpectedly: %.2f", amountPaid)
	}

	// overpay should be capped to total_amount
	_, err = db.Exec(`INSERT INTO wx_payment_orders(
id, out_trade_no, tenant_bill_id, room_number, bill_date, openid, amount_fen, currency, description, trade_state, created_at, updated_at
) VALUES(?, ?, ?, ?, ?, ?, ?, 'CNY', ?, 'CREATED', ?, ?)`,
		"order-2", "otn-2", "bill-1", "101", "2026-05", "mock_openid", 100000, "over payment", now, now)
	if err != nil {
		t.Fatalf("seed overpay order failed: %v", err)
	}
	if err = service.ApplyPaymentByOutTradeNo("otn-2", "wx_tx_2", time.Now().UTC()); err != nil {
		t.Fatalf("apply over payment failed: %v", err)
	}
	if err = db.QueryRow(`SELECT amount_paid, status FROM tenants WHERE id = ?`, "bill-1").Scan(&amountPaid, &status); err != nil {
		t.Fatalf("query tenant after over payment failed: %v", err)
	}
	if amountPaid != 1000 {
		t.Fatalf("expected capped amount_paid=1000 got %.2f", amountPaid)
	}
	if status != "已缴" {
		t.Fatalf("expected status=已缴 got %s", status)
	}
}

func TestReminderService_RunAndRetry(t *testing.T) {
	tempDir := t.TempDir()
	db := database.OpenDB(filepath.Join(tempDir, "reminder.db"))
	defer db.Close()
	database.InitSchema(db)
	database.MigrateSchema(db)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO tenants(
id, room_number, name, phone, rent_amount, water_reading, electricity_reading, water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at, monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bill-r-1", "201", "Bob", "13800000001", 1000, 0, 0, 0, 0, 1000, 0, "月度", "月度", "待缴", "2026-05", "2026-05-01", now, now, 0, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("seed tenant failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO mp_user_bindings(id, openid, room_number, tenant_name, subscribed, subscribed_template_id, created_at, updated_at) VALUES(?, ?, ?, ?, 1, ?, ?, ?)`,
		"bind-1", "mock_openid_1", "201", "Bob", "tpl_1", now, now)
	if err != nil {
		t.Fatalf("seed binding failed: %v", err)
	}

	wechat := NewWeChatPayService(WeChatServiceConfig{MockMode: true, DefaultTemplateID: "tpl_1"})
	reminder := NewReminderService(db, wechat, "tpl_1", 60)
	sent, failed, err := reminder.RunDueReminders(20)
	if err != nil {
		t.Fatalf("run due reminders failed: %v", err)
	}
	if sent != 1 || failed != 0 {
		t.Fatalf("expected sent=1 failed=0 got sent=%d failed=%d", sent, failed)
	}

	// Seed one failed reminder and verify retry success in mock mode.
	nextRetryAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO mp_reminder_logs(
id, openid, room_number, tenant_bill_id, template_id, amount_fen, status, failure_reason, retry_count, next_retry_at, created_at, updated_at
) VALUES(?, ?, ?, ?, ?, ?, 'FAILED', ?, ?, ?, ?, ?)`,
		"log-1", "mock_openid_1", "201", "bill-r-1", "tpl_1", 100000, "temporary error", 1, nextRetryAt, now, now)
	if err != nil {
		t.Fatalf("seed failed reminder log failed: %v", err)
	}
	sent, failed, err = reminder.RetryFailedReminders(20)
	if err != nil {
		t.Fatalf("retry reminders failed: %v", err)
	}
	if sent != 1 || failed != 0 {
		t.Fatalf("expected retry sent=1 failed=0 got sent=%d failed=%d", sent, failed)
	}
}
