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

// InitSchema creates the base tenants table; feature tables and later columns live in MigrateSchema.
func InitSchema(db *sql.DB) {
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
}
