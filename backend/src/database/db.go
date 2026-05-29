package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func OpenDB(path string) *sql.DB {
	// busy_timeout：避免并发写时 "database is locked" 直接失败
	// journal_mode=WAL：提升并发读写体验
	dsn := path + "?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=ON"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatal(err)
	}
	// SQLite 通常建议限制连接数
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	return db
}

func InitSchema(db *sql.DB) {
	// Create the tenants table with all required fields
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS tenants (
  id TEXT PRIMARY KEY NOT NULL,
  room_number TEXT NOT NULL,
  name TEXT NOT NULL,
  phone TEXT,
  id_card TEXT,
  check_in_date TEXT,
  deposit REAL,
  rent_amount REAL NOT NULL,
  water_reading REAL NOT NULL DEFAULT 0,
  electricity_reading REAL NOT NULL DEFAULT 0,
  water_bill REAL NOT NULL DEFAULT 0,
  electricity_bill REAL NOT NULL DEFAULT 0,
  total_amount REAL NOT NULL DEFAULT 0,
  amount_paid REAL NOT NULL DEFAULT 0,
  rent_cycle TEXT DEFAULT '月度',
  utility_cycle TEXT DEFAULT '月度',
  status TEXT NOT NULL DEFAULT '待缴',
  date TEXT NOT NULL, -- Format YYYY-MM
  recorded_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  monthly_income REAL NOT NULL DEFAULT 0,
  annual_income REAL NOT NULL DEFAULT 0,
  water_elec_income REAL NOT NULL DEFAULT 0,
  monthly_water_elec_income REAL NOT NULL DEFAULT 0,
  annual_water_elec_income REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_room_number ON tenants(room_number);
CREATE INDEX IF NOT EXISTS idx_date ON tenants(date);
CREATE INDEX IF NOT EXISTS idx_status ON tenants(status);
`)
	if err != nil {
		log.Fatal(err)
	}

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
		log.Fatal(err)
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
		log.Fatal(err)
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
		log.Fatal(err)
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
		log.Fatal(err)
	}
}
