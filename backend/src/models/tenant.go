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
	RentAmount             float64   `json:"rentAmount"`
	WaterReading           float64   `json:"waterReading"`
	ElectricityReading     float64   `json:"electricityReading"`
	WaterBill              float64   `json:"waterBill"`
	ElectricityBill        float64   `json:"electricityBill"`
	TotalAmount            float64   `json:"totalAmount"`
	AmountPaid             float64   `json:"amountPaid"`
	RentCycle              string    `json:"rentCycle"`
	UtilityCycle           string    `json:"utilityCycle"`
	Status                 string    `json:"status"`
	Date                   string    `json:"date"` // Format YYYY-MM
	RecordedAt             *string   `json:"recordedAt,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	MonthlyIncome          float64   `json:"monthlyIncome"`
	AnnualIncome           float64   `json:"annualIncome"`
	WaterElecIncome        float64   `json:"waterElecIncome"`
	MonthlyWaterElecIncome float64   `json:"monthlyWaterElecIncome"`
	AnnualWaterElecIncome  float64   `json:"annualWaterElecIncome"`
}
