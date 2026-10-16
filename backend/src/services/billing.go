package services

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"rentadmin/src/models"
	"rentadmin/src/utils"
)

// ErrNegativeUsage means a meter reading is lower than the previous period's reading.
var ErrNegativeUsage = errors.New("meter reading is lower than previous period")

// RentCycleMultiplier converts a rent cycle into the number of months billed per period.
func RentCycleMultiplier(cycle string) float64 {
	switch cycle {
	case "季度":
		return 3
	case "半年", "半年度":
		return 6
	case "年度", "年度缴纳", "年":
		return 12
	default: // 月度 / 月付 / 月
		return 1
	}
}

// PaymentStatusFor derives the payment status from paid and total amounts.
func PaymentStatusFor(amountPaid, totalAmount float64) string {
	switch {
	case amountPaid >= totalAmount:
		return "已缴"
	case amountPaid > 0:
		return "部分缴纳"
	default:
		return "待缴"
	}
}

// RoundYuan rounds an amount to whole fen to avoid float drift.
func RoundYuan(v float64) float64 {
	return math.Round(v*100) / 100
}

// YuanToFen converts yuan to fen with rounding (0.29 -> 29, not 28).
func YuanToFen(v float64) int64 {
	return int64(math.Round(v * 100))
}

// monthIndex converts "YYYY-MM" (or a longer "YYYY-MM-DD") into a month count; ok is false for malformed input.
func monthIndex(date string) (int, bool) {
	if len(date) < 7 {
		return 0, false
	}
	parsed, err := time.Parse("2006-01", date[:7])
	if err != nil {
		return 0, false
	}
	return parsed.Year()*12 + int(parsed.Month()) - 1, true
}

// RentDueFor returns the rent charged in billMonth. Rent for a whole cycle is charged in the first
// month of each cycle, counted from anchorMonth (the move-in month); other months carry no rent.
// Example: anchor 2026-01, 季度 -> 2026-01/04/07/10 charge rent×3, every other month 0.
func RentDueFor(rentAmount float64, cycle, anchorMonth, billMonth string) float64 {
	multiplier := int(RentCycleMultiplier(cycle))
	bill, okBill := monthIndex(billMonth)
	anchor, okAnchor := monthIndex(anchorMonth)
	if multiplier > 1 && okBill && okAnchor {
		offset := bill - anchor
		if offset < 0 || offset%multiplier != 0 {
			return 0
		}
	}
	return RoundYuan(rentAmount * float64(multiplier))
}

func formatMonth(index int) string {
	return fmt.Sprintf("%04d-%02d", index/12, index%12+1)
}

// CollectionMonthFor returns the month in which a bill for billMonth is collected: the first
// collection month (anchor + k × cycle) on or after billMonth. Rent is prepaid in that month and
// utilities metered since the previous collection are settled with it.
// Example: anchor 2026-01, 季度 -> bills of 2026-02/03/04 are all collected in 2026-04.
func CollectionMonthFor(cycle, anchorMonth, billMonth string) string {
	multiplier := int(RentCycleMultiplier(cycle))
	bill, okBill := monthIndex(billMonth)
	anchor, okAnchor := monthIndex(anchorMonth)
	if multiplier <= 1 || !okBill || !okAnchor || bill < anchor {
		return billMonth
	}
	offset := bill - anchor
	cycles := (offset + multiplier - 1) / multiplier
	return formatMonth(anchor + cycles*multiplier)
}

// rentAnchorMonth is the tenant's check-in month, or else the month of this tenant's first bill in the room.
func rentAnchorMonth(db *sql.DB, t *models.TenantRecord) (string, error) {
	if t.CheckInDate != nil {
		if _, ok := monthIndex(*t.CheckInDate); ok {
			return (*t.CheckInDate)[:7], nil
		}
	}
	var first sql.NullString
	err := db.QueryRow(`SELECT MIN(date) FROM tenants WHERE room_number = ? AND name = ?`, t.RoomNumber, t.Name).Scan(&first)
	if err != nil {
		return "", err
	}
	if !first.Valid || first.String > t.Date {
		return t.Date, nil
	}
	return first.String, nil
}

// CalculateBill derives every computed field of a monthly bill:
//   - utility bills from the previous period's readings when both are 0 (the client sends 0 to request this);
//   - rentDue from the rent cycle when recomputeRent is set (bills imported from Excel keep their own);
//   - total = rentDue + water + electricity + extraFee, then clamped amountPaid, status and income fields.
func CalculateBill(db *sql.DB, t *models.TenantRecord, recomputeRent bool) error {
	if t.WaterBill < 0 || t.ElectricityBill < 0 || t.ExtraFee < 0 {
		return fmt.Errorf("%w (water bill %.2f, electricity bill %.2f, extra fee %.2f)", ErrNegativeUsage, t.WaterBill, t.ElectricityBill, t.ExtraFee)
	}
	if t.WaterBill == 0 && t.ElectricityBill == 0 {
		var prevWater, prevElec float64
		err := db.QueryRow(`SELECT water_reading, electricity_reading FROM tenants WHERE room_number = ? AND date < ? ORDER BY date DESC LIMIT 1`,
			t.RoomNumber, t.Date).Scan(&prevWater, &prevElec)
		switch {
		case err == nil:
			waterUsage := t.WaterReading - prevWater
			elecUsage := t.ElectricityReading - prevElec
			if waterUsage < 0 || elecUsage < 0 {
				return fmt.Errorf("%w (water %.1f -> %.1f, electricity %.1f -> %.1f)", ErrNegativeUsage, prevWater, t.WaterReading, prevElec, t.ElectricityReading)
			}
			t.WaterBill = RoundYuan(waterUsage * models.WATER_UNIT_PRICE)
			t.ElectricityBill = RoundYuan(elecUsage * models.ELEC_UNIT_PRICE)
		case errors.Is(err, sql.ErrNoRows):
			// First period for this room: readings are the baseline, no utility charge.
		default:
			return err
		}
	}

	anchor, err := rentAnchorMonth(db, t)
	if err != nil {
		return err
	}
	if recomputeRent {
		t.RentDue = RentDueFor(t.RentAmount, t.RentCycle, anchor, t.Date)
	}
	t.DueMonth = CollectionMonthFor(t.RentCycle, anchor, t.Date)

	t.TotalAmount = math.Max(RoundYuan(t.RentDue+t.WaterBill+t.ElectricityBill+t.ExtraFee), 0)
	t.AmountPaid = math.Min(math.Max(RoundYuan(t.AmountPaid), 0), t.TotalAmount)
	t.Status = PaymentStatusFor(t.AmountPaid, t.TotalAmount)

	t.MonthlyIncome = t.AmountPaid
	t.AnnualIncome = t.AmountPaid
	t.WaterElecIncome = t.WaterBill + t.ElectricityBill
	t.MonthlyWaterElecIncome = t.WaterElecIncome
	t.AnnualWaterElecIncome = t.WaterElecIncome
	return nil
}

// BackfillDueMonths fills due_month for bills created before the column existed. Amounts are untouched.
func BackfillDueMonths(db *sql.DB) (int, error) {
	bills, err := QueryTenants(db, `WHERE due_month = ''`)
	if err != nil {
		return 0, err
	}
	for _, t := range bills {
		anchor, err := rentAnchorMonth(db, t)
		if err != nil {
			return 0, err
		}
		if _, err := db.Exec(`UPDATE tenants SET due_month = ? WHERE id = ?`, CollectionMonthFor(t.RentCycle, anchor, t.Date), t.ID); err != nil {
			return 0, err
		}
	}
	return len(bills), nil
}

// RoomPaymentResult reports how a room-level payment was spread over bills.
type RoomPaymentResult struct {
	RoomNumber string             `json:"roomNumber"`
	Applied    map[string]float64 `json:"applied"`   // bill id -> amount applied
	Unapplied  float64            `json:"unapplied"` // left over after every unpaid bill was settled
}

// RegisterRoomPayment applies amount to a room's unpaid bills, oldest due month first
// (then bill month), so one collection settles the cycle's rent and the accumulated utilities.
func RegisterRoomPayment(db *sql.DB, roomNumber string, amount float64) (*RoomPaymentResult, error) {
	if amount <= 0 {
		return nil, errors.New("payment amount must be positive")
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT id, total_amount, amount_paid FROM tenants
WHERE room_number = ? AND amount_paid < total_amount ORDER BY due_month, date`, roomNumber)
	if err != nil {
		return nil, err
	}
	type openBill struct {
		id          string
		total, paid float64
	}
	var open []openBill
	for rows.Next() {
		var b openBill
		if err := rows.Scan(&b.id, &b.total, &b.paid); err != nil {
			rows.Close()
			return nil, err
		}
		open = append(open, b)
	}
	rows.Close()
	if len(open) == 0 {
		return nil, fmt.Errorf("%w: room %s has no unpaid bills", ErrTenantNotFound, roomNumber)
	}

	result := &RoomPaymentResult{RoomNumber: roomNumber, Applied: map[string]float64{}}
	remaining := RoundYuan(amount)
	for _, b := range open {
		if remaining <= 0 {
			break
		}
		part := math.Min(remaining, RoundYuan(b.total-b.paid))
		if err := applyPaidAmount(tx, b.id, b.total, b.paid+part); err != nil {
			return nil, err
		}
		result.Applied[b.id] = part
		remaining = RoundYuan(remaining - part)
	}
	result.Unapplied = remaining
	return result, tx.Commit()
}

// GenerateMonthlyBillsResult reports what GenerateMonthlyBills did.
type GenerateMonthlyBillsResult struct {
	Month   string   `json:"month"`
	Created []string `json:"created"` // room numbers that got a new bill
	Skipped int      `json:"skipped"` // rooms that already had a bill for the month
}

// GenerateMonthlyBills creates a bill for month for every room whose latest earlier bill exists and
// that has no bill for month yet. Tenant details, rent settings and meter readings are carried over,
// so utilities stay 0 until the month's readings are entered.
func GenerateMonthlyBills(db *sql.DB, month string) (*GenerateMonthlyBillsResult, error) {
	if _, ok := monthIndex(month); !ok || len(month) != 7 {
		return nil, fmt.Errorf("invalid month %q, expected YYYY-MM", month)
	}
	result := &GenerateMonthlyBillsResult{Month: month, Created: make([]string, 0)}

	if err := db.QueryRow(`SELECT COUNT(DISTINCT room_number) FROM tenants WHERE date = ?`, month).Scan(&result.Skipped); err != nil {
		return nil, err
	}
	latest, err := QueryTenants(db, `t WHERE t.date = (SELECT MAX(date) FROM tenants WHERE room_number = t.room_number AND date < ?)
AND NOT EXISTS (SELECT 1 FROM tenants WHERE room_number = t.room_number AND date = ?)
ORDER BY t.room_number`, month, month)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	for _, prev := range latest {
		bill := &models.TenantRecord{
			ID: utils.NewID("t_"), RoomNumber: prev.RoomNumber, Name: prev.Name, Phone: prev.Phone,
			IDCard: prev.IDCard, CheckInDate: prev.CheckInDate, Deposit: prev.Deposit,
			RentAmount: prev.RentAmount, RentCycle: prev.RentCycle, UtilityCycle: prev.UtilityCycle,
			WaterReading: prev.WaterReading, ElectricityReading: prev.ElectricityReading,
			Date: month, CreatedAt: now, UpdatedAt: now,
		}
		if err := CalculateBill(db, bill, true); err != nil {
			return nil, fmt.Errorf("room %s: %w", prev.RoomNumber, err)
		}
		if err := InsertTenant(db, bill); err != nil {
			return nil, fmt.Errorf("room %s: %w", prev.RoomNumber, err)
		}
		result.Created = append(result.Created, prev.RoomNumber)
	}
	return result, nil
}

// RegisterPayment atomically adds amount (yuan) to a bill's amount_paid, capped at total_amount.
func RegisterPayment(db *sql.DB, id string, amount float64) (*models.TenantRecord, error) {
	if amount <= 0 {
		return nil, errors.New("payment amount must be positive")
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var total, paid float64
	if err := tx.QueryRow(`SELECT total_amount, amount_paid FROM tenants WHERE id = ?`, id).Scan(&total, &paid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	if err := applyPaidAmount(tx, id, total, paid+amount); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetTenantByID(db, id)
}

// applyPaidAmount writes a new (clamped) amount_paid plus the derived status and income fields.
func applyPaidAmount(tx *sql.Tx, id string, total, newPaid float64) error {
	newPaid = math.Min(math.Max(RoundYuan(newPaid), 0), total)
	_, err := tx.Exec(`UPDATE tenants SET amount_paid = ?, status = ?, monthly_income = ?, annual_income = ?, updated_at = ? WHERE id = ?`,
		newPaid, PaymentStatusFor(newPaid, total), newPaid, newPaid, formatDBTime(time.Now()), id)
	return err
}
