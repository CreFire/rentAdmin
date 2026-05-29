package database

import (
	"database/sql"
	"log"
)

func MigrateSchema(db *sql.DB) {
	// Add columns for monthly and annual income tracking if they don't exist
	// First, check if the columns exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tenants') WHERE name='monthly_income'").Scan(&count)
	if err != nil {
		log.Printf("Error checking if monthly_income column exists: %v", err)
	} else if count == 0 {
		// Add monthly_income column
		_, err = db.Exec("ALTER TABLE tenants ADD COLUMN monthly_income REAL NOT NULL DEFAULT 0")
		if err != nil {
			log.Printf("Error adding monthly_income column: %v", err)
		}
	}

	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tenants') WHERE name='annual_income'").Scan(&count)
	if err != nil {
		log.Printf("Error checking if annual_income column exists: %v", err)
	} else if count == 0 {
		// Add annual_income column
		_, err = db.Exec("ALTER TABLE tenants ADD COLUMN annual_income REAL NOT NULL DEFAULT 0")
		if err != nil {
			log.Printf("Error adding annual_income column: %v", err)
		}
	}

	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tenants') WHERE name='water_elec_income'").Scan(&count)
	if err != nil {
		log.Printf("Error checking if water_elec_income column exists: %v", err)
	} else if count == 0 {
		// Add water_elec_income column
		_, err = db.Exec("ALTER TABLE tenants ADD COLUMN water_elec_income REAL NOT NULL DEFAULT 0")
		if err != nil {
			log.Printf("Error adding water_elec_income column: %v", err)
		}
	}

	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tenants') WHERE name='monthly_water_elec_income'").Scan(&count)
	if err != nil {
		log.Printf("Error checking if monthly_water_elec_income column exists: %v", err)
	} else if count == 0 {
		// Add monthly_water_elec_income column
		_, err = db.Exec("ALTER TABLE tenants ADD COLUMN monthly_water_elec_income REAL NOT NULL DEFAULT 0")
		if err != nil {
			log.Printf("Error adding monthly_water_elec_income column: %v", err)
		}
	}

	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tenants') WHERE name='annual_water_elec_income'").Scan(&count)
	if err != nil {
		log.Printf("Error checking if annual_water_elec_income column exists: %v", err)
	} else if count == 0 {
		// Add annual_water_elec_income column
		_, err = db.Exec("ALTER TABLE tenants ADD COLUMN annual_water_elec_income REAL NOT NULL DEFAULT 0")
		if err != nil {
			log.Printf("Error adding annual_water_elec_income column: %v", err)
		}
	}

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
		log.Printf("Error migrating wx_payment_orders: %v", err)
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
		log.Printf("Error migrating wx_pay_notify_logs: %v", err)
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
		log.Printf("Error migrating mp_user_bindings: %v", err)
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
		log.Printf("Error migrating mp_reminder_logs: %v", err)
	}
}
