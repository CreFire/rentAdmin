package services

import (
	"database/sql"
	"errors"
	"time"

	"rentadmin/src/models"
)

// ErrTenantNotFound is returned when a tenant bill id does not exist.
var ErrTenantNotFound = errors.New("tenant not found")

const tenantColumns = `id, room_number, name, phone, id_card, check_in_date, deposit, rent_amount, water_reading, electricity_reading,
water_bill, electricity_bill, total_amount, amount_paid, rent_cycle, utility_cycle, status, date, recorded_at, created_at, updated_at,
monthly_income, annual_income, water_elec_income, monthly_water_elec_income, annual_water_elec_income, rent_due, extra_fee, due_month`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

// dbTimeLayouts covers RFC3339 strings and the format go-sqlite3 uses for time.Time values written by older code.
var dbTimeLayouts = []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"}

func parseDBTime(value string) time.Time {
	for _, layout := range dbTimeLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func formatDBTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func scanTenant(row rowScanner) (*models.TenantRecord, error) {
	var t models.TenantRecord
	var phone, rentCycle, utilityCycle sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.RoomNumber, &t.Name, &phone, &t.IDCard, &t.CheckInDate, &t.Deposit, &t.RentAmount,
		&t.WaterReading, &t.ElectricityReading, &t.WaterBill, &t.ElectricityBill, &t.TotalAmount, &t.AmountPaid,
		&rentCycle, &utilityCycle, &t.Status, &t.Date, &t.RecordedAt, &createdAt, &updatedAt,
		&t.MonthlyIncome, &t.AnnualIncome, &t.WaterElecIncome, &t.MonthlyWaterElecIncome, &t.AnnualWaterElecIncome, &t.RentDue, &t.ExtraFee, &t.DueMonth)
	if err != nil {
		return nil, err
	}
	t.Phone = phone.String
	t.RentCycle = rentCycle.String
	t.UtilityCycle = utilityCycle.String
	t.CreatedAt = parseDBTime(createdAt)
	t.UpdatedAt = parseDBTime(updatedAt)
	return &t, nil
}

// QueryTenants runs `SELECT <all columns> FROM tenants <clause>` and returns every row.
func QueryTenants(db *sql.DB, clause string, args ...interface{}) ([]*models.TenantRecord, error) {
	rows, err := db.Query(`SELECT `+tenantColumns+` FROM tenants `+clause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*models.TenantRecord, 0)
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// GetTenantByID returns ErrTenantNotFound when the id does not exist.
func GetTenantByID(db *sql.DB, id string) (*models.TenantRecord, error) {
	t, err := scanTenant(db.QueryRow(`SELECT `+tenantColumns+` FROM tenants WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTenantNotFound
	}
	return t, err
}

func InsertTenant(db *sql.DB, t *models.TenantRecord) error {
	_, err := db.Exec(`INSERT INTO tenants(`+tenantColumns+`) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.RoomNumber, t.Name, t.Phone, t.IDCard, t.CheckInDate, t.Deposit, t.RentAmount, t.WaterReading, t.ElectricityReading,
		t.WaterBill, t.ElectricityBill, t.TotalAmount, t.AmountPaid, t.RentCycle, t.UtilityCycle, t.Status, t.Date, t.RecordedAt,
		formatDBTime(t.CreatedAt), formatDBTime(t.UpdatedAt),
		t.MonthlyIncome, t.AnnualIncome, t.WaterElecIncome, t.MonthlyWaterElecIncome, t.AnnualWaterElecIncome, t.RentDue, t.ExtraFee, t.DueMonth)
	return err
}

// UpdateTenant overwrites every mutable column of the row identified by id (room_number, date and created_at are kept).
func UpdateTenant(db *sql.DB, id string, t *models.TenantRecord) (int64, error) {
	result, err := db.Exec(`UPDATE tenants SET name=?, phone=?, id_card=?, check_in_date=?, deposit=?, rent_amount=?, water_reading=?, electricity_reading=?,
water_bill=?, electricity_bill=?, total_amount=?, amount_paid=?, rent_cycle=?, utility_cycle=?, status=?, recorded_at=?, updated_at=?,
monthly_income=?, annual_income=?, water_elec_income=?, monthly_water_elec_income=?, annual_water_elec_income=?, rent_due=?, extra_fee=?, due_month=? WHERE id = ?`,
		t.Name, t.Phone, t.IDCard, t.CheckInDate, t.Deposit, t.RentAmount, t.WaterReading, t.ElectricityReading,
		t.WaterBill, t.ElectricityBill, t.TotalAmount, t.AmountPaid, t.RentCycle, t.UtilityCycle, t.Status, t.RecordedAt,
		formatDBTime(t.UpdatedAt),
		t.MonthlyIncome, t.AnnualIncome, t.WaterElecIncome, t.MonthlyWaterElecIncome, t.AnnualWaterElecIncome, t.RentDue, t.ExtraFee, t.DueMonth, id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
