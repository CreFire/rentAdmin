package services

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rentadmin/src/utils"
)

const (
	// reminderCooldown is the minimum gap between two reminders to the same openid.
	reminderCooldown = 24 * time.Hour
	// maxReminderRetries caps retries of a failed reminder before it is abandoned.
	maxReminderRetries = 3
)

type ReminderService struct {
	DB                *sql.DB
	WeChat            *WeChatPayService
	DefaultTemplateID string
	RetryDelay        time.Duration
}

func NewReminderService(db *sql.DB, wechat *WeChatPayService, defaultTemplateID string, retryDelaySeconds int64) *ReminderService {
	if retryDelaySeconds <= 0 {
		retryDelaySeconds = 300
	}

	return &ReminderService{
		DB:                db,
		WeChat:            wechat,
		DefaultTemplateID: defaultTemplateID,
		RetryDelay:        time.Duration(retryDelaySeconds) * time.Second,
	}
}

func (s *ReminderService) RecordSubscription(openid, templateID string) error {
	if openid == "" {
		return errors.New("openid is required")
	}
	if templateID == "" {
		templateID = s.DefaultTemplateID
	}
	if templateID == "" {
		return errors.New("template id is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`
UPDATE mp_user_bindings
SET subscribed = 1, subscribed_template_id = ?, subscribed_at = ?, updated_at = ?
WHERE openid = ?`,
		templateID, now, now, openid)
	return err
}

func (s *ReminderService) RunDueReminders(limit int) (int, int, error) {
	if limit <= 0 {
		limit = 100
	}

	// One reminder per bound room, covering every bill whose collection month has arrived
	// (quarterly / half-year tenants are not chased for utilities between collections).
	rows, err := s.DB.Query(`
SELECT b.openid, b.room_number, COALESCE(b.subscribed_template_id, ''),
  (SELECT t2.id FROM tenants t2 WHERE t2.room_number = b.room_number AND t2.amount_paid < t2.total_amount AND t2.due_month <= ? ORDER BY t2.date DESC LIMIT 1),
  MAX(t.name), MAX(t.due_month), SUM(t.total_amount), SUM(t.amount_paid)
FROM mp_user_bindings b
JOIN tenants t ON t.room_number = b.room_number
WHERE b.subscribed = 1
AND (b.last_reminded_at IS NULL OR b.last_reminded_at <= ?)
AND t.amount_paid < t.total_amount
AND t.due_month <= ?
GROUP BY b.openid, b.room_number, b.subscribed_template_id
LIMIT ?`, currentMonth(), time.Now().UTC().Add(-reminderCooldown).Format(time.RFC3339), currentMonth(), limit)
	if err != nil {
		return 0, 0, err
	}

	type dueReminder struct {
		openid     string
		roomNumber string
		templateID string
		tenantBill string
		tenantName string
		billDate   string
		total      float64
		paid       float64
	}
	reminders := make([]dueReminder, 0)
	for rows.Next() {
		var item dueReminder
		if err = rows.Scan(&item.openid, &item.roomNumber, &item.templateID, &item.tenantBill, &item.tenantName, &item.billDate, &item.total, &item.paid); err != nil {
			_ = rows.Close()
			return 0, 0, err
		}
		reminders = append(reminders, item)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	_ = rows.Close()

	sentCount := 0
	failCount := 0
	now := time.Now().UTC()
	for _, item := range reminders {
		openid := item.openid
		roomNumber := item.roomNumber
		templateID := item.templateID
		tenantBillID := item.tenantBill
		tenantName := item.tenantName
		billDate := item.billDate
		totalAmount := item.total
		amountPaid := item.paid

		if templateID == "" {
			templateID = s.DefaultTemplateID
		}
		// Every attempt starts the cooldown; failures are handled by RetryFailedReminders.
		_, _ = s.DB.Exec(`UPDATE mp_user_bindings SET last_reminded_at = ?, updated_at = ? WHERE openid = ?`,
			now.Format(time.RFC3339), now.Format(time.RFC3339), openid)

		if templateID == "" {
			failCount++
			_ = s.recordReminderResult(openid, roomNumber, tenantBillID, "", YuanToFen(totalAmount-amountPaid), "FAILED", "missing template id", 1, now.Add(s.RetryDelay))
			continue
		}

		data := map[string]string{
			"thing1":  tenantName,
			"thing2":  roomNumber,
			"date3":   billDate,
			"amount4": fmt.Sprintf("%.2f", totalAmount-amountPaid),
		}
		sendErr := s.WeChat.SendSubscribeMessage(openid, templateID, data)
		if sendErr != nil {
			failCount++
			_ = s.recordReminderResult(openid, roomNumber, tenantBillID, templateID, YuanToFen(totalAmount-amountPaid), "FAILED", sendErr.Error(), 1, now.Add(s.RetryDelay))
			continue
		}

		sentCount++
		_ = s.recordReminderResult(openid, roomNumber, tenantBillID, templateID, YuanToFen(totalAmount-amountPaid), "SENT", "", 0, time.Time{})
		markSubscriptionUsed(s.DB, openid, now)
	}

	return sentCount, failCount, nil
}

func (s *ReminderService) RetryFailedReminders(limit int) (int, int, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.DB.Query(`
SELECT id, openid, room_number, tenant_bill_id, template_id, amount_fen, retry_count
FROM mp_reminder_logs
WHERE status = 'FAILED'
AND retry_count <= ?
AND next_retry_at IS NOT NULL
AND next_retry_at <= ?
ORDER BY next_retry_at ASC
LIMIT ?`, maxReminderRetries, time.Now().UTC().Format(time.RFC3339), limit)
	if err != nil {
		return 0, 0, err
	}

	type retryItem struct {
		id         string
		openid     string
		roomNumber string
		tenantBill string
		templateID string
		amountFen  int64
		retryCount int
	}
	items := make([]retryItem, 0)
	for rows.Next() {
		var item retryItem
		if err = rows.Scan(&item.id, &item.openid, &item.roomNumber, &item.tenantBill, &item.templateID, &item.amountFen, &item.retryCount); err != nil {
			_ = rows.Close()
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	_ = rows.Close()

	sentCount := 0
	failCount := 0
	now := time.Now().UTC()
	for _, item := range items {
		id := item.id
		openid := item.openid
		roomNumber := item.roomNumber
		templateID := item.templateID
		amountFen := item.amountFen
		retryCount := item.retryCount

		data := map[string]string{
			"thing1":  roomNumber,
			"amount4": fmt.Sprintf("%.2f", float64(amountFen)/100.0),
		}
		sendErr := s.WeChat.SendSubscribeMessage(openid, templateID, data)
		if sendErr != nil {
			failCount++
			retryCount++
			status := "FAILED"
			if retryCount > maxReminderRetries {
				status = "ABANDONED"
			}
			_, _ = s.DB.Exec(`UPDATE mp_reminder_logs SET status = ?, retry_count = ?, failure_reason = ?, next_retry_at = ?, updated_at = ? WHERE id = ?`,
				status, retryCount, sendErr.Error(), now.Add(s.RetryDelay).Format(time.RFC3339), now.Format(time.RFC3339), id)
			continue
		}

		sentCount++
		_, _ = s.DB.Exec(`UPDATE mp_reminder_logs SET status = 'SENT', sent_at = ?, updated_at = ?, failure_reason = '' WHERE id = ?`,
			now.Format(time.RFC3339), now.Format(time.RFC3339), id)
		markSubscriptionUsed(s.DB, openid, now)
	}

	return sentCount, failCount, nil
}

func (s *ReminderService) recordReminderResult(openid, roomNumber, tenantBillID, templateID string, amountFen int64, status, reason string, retryCount int, nextRetryAt time.Time) error {
	id := utils.NewID("rem_")
	now := time.Now().UTC()
	var nextRetry interface{}
	if !nextRetryAt.IsZero() {
		nextRetry = nextRetryAt.Format(time.RFC3339)
	}

	_, err := s.DB.Exec(`INSERT INTO mp_reminder_logs(
id, openid, room_number, tenant_bill_id, template_id, amount_fen, status, failure_reason, retry_count, next_retry_at, sent_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, openid, roomNumber, tenantBillID, templateID, amountFen, status, reason, retryCount, nextRetry,
		func() interface{} {
			if status == "SENT" {
				return now.Format(time.RFC3339)
			}
			return nil
		}(),
		now.Format(time.RFC3339), now.Format(time.RFC3339))
	return err
}

// currentMonth is the collection month "now" in China local time (UTC+8), where the business operates.
func currentMonth() string {
	return time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01")
}

// markSubscriptionUsed clears the flag after a successful send: WeChat one-time
// subscriptions allow a single message per user authorization.
func markSubscriptionUsed(db *sql.DB, openid string, now time.Time) {
	_, _ = db.Exec(`UPDATE mp_user_bindings SET subscribed = 0, updated_at = ? WHERE openid = ?`, now.Format(time.RFC3339), openid)
}
