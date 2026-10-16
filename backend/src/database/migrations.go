package database

import (
	"database/sql"
	"log"
)

// addedColumn is a tenants column introduced after the initial release. Backfill runs once, right after the column is added.
type addedColumn struct {
	name     string
	ddl      string // column type and default; REAL NOT NULL DEFAULT 0 when empty
	backfill string
}

var addedTenantColumns = []addedColumn{
	{name: "monthly_income"},
	{name: "annual_income"},
	{name: "water_elec_income"},
	{name: "monthly_water_elec_income"},
	{name: "annual_water_elec_income"},
	{name: "extra_fee"},
	// Existing bills keep their totals: whatever is not utilities counts as this period's rent.
	{name: "rent_due", backfill: "UPDATE tenants SET rent_due = MAX(total_amount - water_bill - electricity_bill, 0)"},
	// Month in which the bill is collected; filled in by services.BackfillDueMonths for existing rows.
	{name: "due_month", ddl: "TEXT NOT NULL DEFAULT ''"},
}

// MigrateSchema applies idempotent schema changes. Any failure aborts startup so the
// server never runs against a half-migrated database.
func MigrateSchema(db *sql.DB) {
	for _, column := range addedTenantColumns {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tenants') WHERE name = ?", column.name).Scan(&count); err != nil {
			log.Fatalf("check column %s: %v", column.name, err)
		}
		if count > 0 {
			continue
		}
		ddl := column.ddl
		if ddl == "" {
			ddl = "REAL NOT NULL DEFAULT 0"
		}
		if _, err := db.Exec("ALTER TABLE tenants ADD COLUMN " + column.name + " " + ddl); err != nil {
			log.Fatalf("add column %s: %v", column.name, err)
		}
		if column.backfill != "" {
			if _, err := db.Exec(column.backfill); err != nil {
				log.Fatalf("backfill column %s: %v", column.name, err)
			}
		}
	}

	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_due_month ON tenants(due_month)"); err != nil {
		log.Fatalf("create idx_due_month: %v", err)
	}

	var err error
	// Payment and mini program related tables for new feature set.
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS wx_payment_orders (
  id TEXT PRIMARY KEY NOT NULL,
  out_trade_no TEXT NOT NULL UNIQUE,
  tenant_bill_id TEXT NOT NULL,
  room_number TEXT NOT NULL,
  bill_date TEXT NOT NULL,
  openid TEXT NOT NULL,
  amount_fen INTEGER NOT NULL,
  currency TEXT NOT NULL DEFAULT 'CNY',
  description TEXT,
  trade_state TEXT NOT NULL DEFAULT 'CREATED',
  wx_transaction_id TEXT,
  prepay_id TEXT,
  notify_processed INTEGER NOT NULL DEFAULT 0,
  paid_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_sync_at TEXT,
  sync_err TEXT
);
CREATE INDEX IF NOT EXISTS idx_wx_payment_orders_tenant_bill ON wx_payment_orders(tenant_bill_id);
CREATE INDEX IF NOT EXISTS idx_wx_payment_orders_openid ON wx_payment_orders(openid);
CREATE INDEX IF NOT EXISTS idx_wx_payment_orders_trade_state ON wx_payment_orders(trade_state);
`)
	if err != nil {
		log.Fatalf("Error migrating wx_payment_orders: %v", err)
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS wx_pay_notify_logs (
  id TEXT PRIMARY KEY NOT NULL,
  out_trade_no TEXT NOT NULL,
  wx_event_id TEXT,
  verify_ok INTEGER NOT NULL DEFAULT 0,
  processed INTEGER NOT NULL DEFAULT 0,
  request_body TEXT,
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wx_notify_event_unique ON wx_pay_notify_logs(wx_event_id);
CREATE INDEX IF NOT EXISTS idx_wx_notify_out_trade_no ON wx_pay_notify_logs(out_trade_no);
`)
	if err != nil {
		log.Fatalf("Error migrating wx_pay_notify_logs: %v", err)
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS mp_user_bindings (
  id TEXT PRIMARY KEY NOT NULL,
  openid TEXT NOT NULL UNIQUE,
  unionid TEXT,
  room_number TEXT NOT NULL,
  tenant_name TEXT,
  subscribed INTEGER NOT NULL DEFAULT 0,
  subscribed_template_id TEXT,
  subscribed_at TEXT,
  last_reminded_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_mp_user_bindings_room ON mp_user_bindings(room_number);
`)
	if err != nil {
		log.Fatalf("Error migrating mp_user_bindings: %v", err)
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS mp_reminder_logs (
  id TEXT PRIMARY KEY NOT NULL,
  openid TEXT NOT NULL,
  room_number TEXT NOT NULL,
  tenant_bill_id TEXT,
  template_id TEXT NOT NULL,
  amount_fen INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'PENDING',
  failure_reason TEXT,
  retry_count INTEGER NOT NULL DEFAULT 0,
  next_retry_at TEXT,
  sent_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_mp_reminder_logs_openid ON mp_reminder_logs(openid);
CREATE INDEX IF NOT EXISTS idx_mp_reminder_logs_status ON mp_reminder_logs(status);
`)
	if err != nil {
		log.Fatalf("Error migrating mp_reminder_logs: %v", err)
	}
}
