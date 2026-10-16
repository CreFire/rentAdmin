package models

import (
	"time"
)

// Constants for utility prices
const (
	WATER_UNIT_PRICE = 5.5
	ELEC_UNIT_PRICE  = 1.2
)

// TenantRecord represents a tenant record in the system
type TenantRecord struct {
	ID                     string    `json:"id"`
	RoomNumber             string    `json:"roomNumber"`
	Name                   string    `json:"name"`
	Phone                  string    `json:"phone"`
	IDCard                 *string   `json:"idCard,omitempty"`
	CheckInDate            *string   `json:"checkInDate,omitempty"`
	Deposit                *float64  `json:"deposit,omitempty"`
	RentAmount             float64   `json:"rentAmount"` // 月租
	RentDue                float64   `json:"rentDue"`    // 本期应收房租：周期首月 = 月租 × 周期倍数，其余月份 0
	WaterReading           float64   `json:"waterReading"`
	ElectricityReading     float64   `json:"electricityReading"`
	WaterBill              float64   `json:"waterBill"`
	ElectricityBill        float64   `json:"electricityBill"`
	ExtraFee               float64   `json:"extraFee"`    // 其他费用（垃圾费、维修费等）
	TotalAmount            float64   `json:"totalAmount"` // rentDue + waterBill + electricityBill + extraFee
	AmountPaid             float64   `json:"amountPaid"`
	RentCycle              string    `json:"rentCycle"`
	UtilityCycle           string    `json:"utilityCycle"`
	Status                 string    `json:"status"`
	Date                   string    `json:"date"`     // 账单月 YYYY-MM（抄表/计费月）
	DueMonth               string    `json:"dueMonth"` // 收款月 YYYY-MM：从账单月起第一个收租月，按缴费周期合并收取
	RecordedAt             *string   `json:"recordedAt,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	MonthlyIncome          float64   `json:"monthlyIncome"`
	AnnualIncome           float64   `json:"annualIncome"`
	WaterElecIncome        float64   `json:"waterElecIncome"`
	MonthlyWaterElecIncome float64   `json:"monthlyWaterElecIncome"`
	AnnualWaterElecIncome  float64   `json:"annualWaterElecIncome"`
}
