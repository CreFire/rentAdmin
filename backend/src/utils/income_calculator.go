package utils

import (
	"database/sql"
	"fmt"
	"log"

	"rentadmin/src/models"
)

// CalculateMonthlyAnnualIncome 计算月度和年度收入统计
func CalculateMonthlyAnnualIncome(db *sql.DB, currentMonth string) error {
	// 获取当前月份的所有记录
	rows, err := db.Query("SELECT id, amount_paid, water_bill, electricity_bill, date FROM tenants WHERE date = ?", currentMonth)
	if err != nil {
		return fmt.Errorf("failed to query records for month %s: %v", currentMonth, err)
	}
	defer rows.Close()

	var records []struct {
		ID              string
		AmountPaid      float64
		WaterBill       float64
		ElectricityBill float64
		Date            string
	}

	for rows.Next() {
		var record struct {
			ID              string
			AmountPaid      float64
			WaterBill       float64
			ElectricityBill float64
			Date            string
		}
		err := rows.Scan(&record.ID, &record.AmountPaid, &record.WaterBill, &record.ElectricityBill, &record.Date)
		if err != nil {
			log.Printf("Error scanning record: %v", err)
			continue
		}
		records = append(records, record)
	}

	// 更新每条记录的收入统计
	for _, record := range records {
		// 计算当月水电总收入
		monthlyWaterElecIncome := record.WaterBill + record.ElectricityBill

		// 计算年度水电总收入（需要查询同一年的所有记录）
		year := record.Date[:4] // 提取年份，例如 "2026-01" -> "2026"
		var annualWaterElecIncome float64
		err := db.QueryRow(
			"SELECT COALESCE(SUM(water_bill + electricity_bill), 0) FROM tenants WHERE room_number IN (SELECT DISTINCT room_number FROM tenants WHERE date LIKE ?) AND date LIKE ?",
			year+"%", year+"%",
		).Scan(&annualWaterElecIncome)
		if err != nil {
			log.Printf("Error calculating annual water/electricity income for record %s: %v", record.ID, err)
			annualWaterElecIncome = monthlyWaterElecIncome
		}

		// 计算年度总收入（房租+水电）
		var annualIncome float64
		err = db.QueryRow(
			"SELECT COALESCE(SUM(amount_paid), 0) FROM tenants WHERE room_number IN (SELECT DISTINCT room_number FROM tenants WHERE date LIKE ?) AND date LIKE ?",
			year+"%", year+"%",
		).Scan(&annualIncome)
		if err != nil {
			log.Printf("Error calculating annual income for record %s: %v", record.ID, err)
			annualIncome = record.AmountPaid
		}

		// 更新记录
		_, err = db.Exec(
			"UPDATE tenants SET monthly_income = ?, annual_income = ?, monthly_water_elec_income = ?, annual_water_elec_income = ? WHERE id = ?",
			record.AmountPaid, annualIncome, monthlyWaterElecIncome, annualWaterElecIncome, record.ID,
		)
		if err != nil {
			log.Printf("Error updating income stats for record %s: %v", record.ID, err)
		}
	}

	return nil
}

// GetIncomeSummary 获取收入汇总信息
func GetIncomeSummary(db *sql.DB, dateFilter string) (*models.TenantRecord, error) {
	query := `
		SELECT 
			COALESCE(SUM(total_amount), 0) AS total_receivable,
			COALESCE(SUM(amount_paid), 0) AS total_received,
			COALESCE(SUM(water_bill + electricity_bill), 0) AS total_utility_bills,
			COALESCE(SUM(CASE WHEN amount_paid > 0 AND amount_paid < total_amount THEN total_amount - amount_paid ELSE 0 END), 0) AS outstanding_balance
		FROM tenants
	`

	var args []interface{}
	if dateFilter != "" {
		query += " WHERE date LIKE ?"
		args = append(args, dateFilter+"%")
	}

	var summary models.TenantRecord
	err := db.QueryRow(query, args...).Scan(
		&summary.TotalAmount,     // 累计应收
		&summary.AmountPaid,      // 累计实收
		&summary.WaterElecIncome, // 水电总收入
		&summary.MonthlyIncome,   // 代收余额（未收金额）
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get income summary: %v", err)
	}

	// 计算代收余额（应为总应收减去总实收，但不能为负数）
	balance := summary.TotalAmount - summary.AmountPaid
	if balance < 0 {
		balance = 0
	}
	summary.MonthlyIncome = balance

	return &summary, nil
}
