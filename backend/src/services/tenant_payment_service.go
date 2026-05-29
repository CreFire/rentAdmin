package services

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type TenantPaymentService struct {
	DB *sql.DB
}

func NewTenantPaymentService(db *sql.DB) *TenantPaymentService {
	return &TenantPaymentService{DB: db}
}

func (s *TenantPaymentService) ApplyPaymentByOutTradeNo(outTradeNo, transactionID string, paidAt time.Time) error {
	if outTradeNo == "" {
		return errors.New("outTradeNo is required")
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var tenantBillID string
	var amountFen int64
	var tradeState string
	var existingTransaction sql.NullString
	err = tx.QueryRow(`SELECT tenant_bill_id, amount_fen, trade_state, wx_transaction_id FROM wx_payment_orders WHERE out_trade_no = ?`,
		outTradeNo).Scan(&tenantBillID, &amountFen, &tradeState, &existingTransaction)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("payment order not found")
		}
		return err
	}

	if tradeState == "SUCCESS" {
		if existingTransaction.Valid && existingTransaction.String != "" && transactionID != "" && existingTransaction.String != transactionID {
			return fmt.Errorf("order already paid with another transaction id")
		}
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		committed = true
		return nil
	}

	var totalAmount, amountPaid float64
	err = tx.QueryRow(`SELECT total_amount, amount_paid FROM tenants WHERE id = ?`, tenantBillID).Scan(&totalAmount, &amountPaid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("tenant bill not found")
		}
		return err
	}

	paymentAmount := float64(amountFen) / 100.0
	if paymentAmount <= 0 {
		return errors.New("invalid payment amount")
	}

	newAmountPaid := amountPaid + paymentAmount
	if newAmountPaid > totalAmount {
		newAmountPaid = totalAmount
	}
	if newAmountPaid < 0 {
		newAmountPaid = 0
	}

	status := "待缴"
	if newAmountPaid >= totalAmount {
		status = "已缴"
	} else if newAmountPaid > 0 {
		status = "部分缴纳"
	}

	now := time.Now().UTC()
	_, err = tx.Exec(`UPDATE tenants SET amount_paid = ?, status = ?, monthly_income = ?, annual_income = ?, updated_at = ? WHERE id = ?`,
		newAmountPaid, status, newAmountPaid, newAmountPaid, now, tenantBillID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE wx_payment_orders SET trade_state = 'SUCCESS', wx_transaction_id = ?, paid_at = ?, notify_processed = 1, updated_at = ? WHERE out_trade_no = ?`,
		transactionID, paidAt.UTC().Format(time.RFC3339), now.Format(time.RFC3339), outTradeNo)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	committed = true
	return nil
}
