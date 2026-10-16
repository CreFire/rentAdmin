package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"rentadmin/src/database"
	"rentadmin/src/handlers"
	"rentadmin/src/models"
	"rentadmin/src/services"

	"github.com/gin-gonic/gin"
)

const testAdminToken = "test-admin-token"

func newTestServer(t *testing.T) (*gin.Engine, *services.AuthService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := database.OpenDB(filepath.Join(t.TempDir(), "test.db"))
	t.Cleanup(func() { db.Close() })
	database.InitSchema(db)
	database.MigrateSchema(db)

	auth := services.NewAuthService("test-secret", 3600)
	wechat := services.NewWeChatPayService(services.WeChatServiceConfig{MockMode: true})
	reminder := services.NewReminderService(db, wechat, "", 60)
	r := gin.New()
	SetupRoutes(r, RouteConfig{AdminToken: testAdminToken, Auth: auth, MockPayment: true},
		&handlers.TenantHandler{DB: db},
		&handlers.ExcelHandler{DB: db},
		&handlers.MPAuthHandler{DB: db, Auth: auth, WeChat: wechat, Reminder: reminder},
		&handlers.WXPayHandler{DB: db, Auth: auth, WeChat: wechat, TenantPayment: services.NewTenantPaymentService(db), Reminder: reminder},
	)
	return r, auth
}

func doJSON(t *testing.T, r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func createBill(t *testing.T, r *gin.Engine, room, date string, water, elec float64) string {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/tenants", testAdminToken, map[string]interface{}{
		"roomNumber": room, "name": "张三", "phone": "13800138001", "rentAmount": 1000, "rentCycle": "月度",
		"waterReading": water, "electricityReading": elec, "date": date,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create bill: %d %s", w.Code, w.Body.String())
	}
	var resp struct{ ID string }
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.ID
}

func listBills(t *testing.T, r *gin.Engine) map[string]models.TenantRecord {
	t.Helper()
	w := doJSON(t, r, http.MethodGet, "/api/tenants", testAdminToken, nil)
	var list []models.TenantRecord
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v %s", err, w.Body.String())
	}
	byID := make(map[string]models.TenantRecord, len(list))
	for _, rec := range list {
		byID[rec.ID] = rec
	}
	return byID
}

func TestAdminRoutesRequireToken(t *testing.T) {
	r, _ := newTestServer(t)
	for _, token := range []string{"", "wrong"} {
		if w := doJSON(t, r, http.MethodGet, "/api/tenants", token, nil); w.Code != http.StatusUnauthorized {
			t.Fatalf("token %q: expected 401, got %d", token, w.Code)
		}
	}
	if w := doJSON(t, r, http.MethodDelete, "/api/tenants", testAdminToken, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("clear without confirm: expected 400, got %d", w.Code)
	}
}

func TestPartialUpdateKeepsOtherFields(t *testing.T) {
	r, _ := newTestServer(t)
	createBill(t, r, "101", "2026-01", 10, 100)
	id := createBill(t, r, "101", "2026-02", 12, 150) // water 2*5.5 + elec 50*1.2 = 71

	w := doJSON(t, r, http.MethodPut, "/api/tenants/"+id, testAdminToken, map[string]interface{}{"amountPaid": 500})
	if w.Code != http.StatusOK {
		t.Fatalf("partial update: %d %s", w.Code, w.Body.String())
	}
	got := listBills(t, r)[id]
	if got.Name != "张三" || got.RentAmount != 1000 || got.WaterReading != 12 || got.TotalAmount != 1071 {
		t.Fatalf("partial update wiped fields: %+v", got)
	}
	if got.AmountPaid != 500 || got.Status != "部分缴纳" {
		t.Fatalf("unexpected payment state: paid=%v status=%s", got.AmountPaid, got.Status)
	}
}

func TestRegisterPaymentAccumulatesAndCaps(t *testing.T) {
	r, _ := newTestServer(t)
	id := createBill(t, r, "102", "2026-01", 10, 100) // first period: rent only, 1000

	for _, amount := range []float64{300, 300} {
		if w := doJSON(t, r, http.MethodPost, "/api/tenants/"+id+"/payments", testAdminToken, map[string]float64{"amount": amount}); w.Code != http.StatusOK {
			t.Fatalf("payment: %d %s", w.Code, w.Body.String())
		}
	}
	if got := listBills(t, r)[id]; got.AmountPaid != 600 || got.Status != "部分缴纳" || got.Name != "张三" {
		t.Fatalf("after 2 payments: %+v", got)
	}
	doJSON(t, r, http.MethodPost, "/api/tenants/"+id+"/payments", testAdminToken, map[string]float64{"amount": 9999})
	if got := listBills(t, r)[id]; got.AmountPaid != 1000 || got.Status != "已缴" {
		t.Fatalf("overpayment not capped: paid=%v status=%s", got.AmountPaid, got.Status)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/tenants/missing/payments", testAdminToken, map[string]float64{"amount": 1}); w.Code != http.StatusNotFound {
		t.Fatalf("missing bill: expected 404, got %d", w.Code)
	}
}

func TestNegativeUsageRejected(t *testing.T) {
	r, _ := newTestServer(t)
	createBill(t, r, "103", "2026-01", 50, 500)
	w := doJSON(t, r, http.MethodPost, "/api/tenants", testAdminToken, map[string]interface{}{
		"roomNumber": "103", "name": "张三", "rentAmount": 1000, "waterReading": 40, "electricityReading": 600, "date": "2026-02",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for decreasing meter reading, got %d %s", w.Code, w.Body.String())
	}
}

func TestMiniProgramBindingAndOwnership(t *testing.T) {
	r, auth := newTestServer(t)
	billID := createBill(t, r, "201", "2026-01", 10, 100)
	tokenA, _, _ := auth.GenerateToken("openid-a")
	tokenB, _, _ := auth.GenerateToken("openid-b")

	if w := doJSON(t, r, http.MethodPost, "/api/mp/bind", tokenA, map[string]string{"roomNumber": "201", "tenantName": "李四"}); w.Code != http.StatusForbidden {
		t.Fatalf("bind with wrong name: expected 403, got %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/mp/bind", tokenA, map[string]string{"roomNumber": "201", "tenantName": "张三"}); w.Code != http.StatusOK {
		t.Fatalf("bind: %d %s", w.Code, w.Body.String())
	}

	w := doJSON(t, r, http.MethodGet, "/api/mp/bills", tokenA, nil)
	if bytes.Contains(w.Body.Bytes(), []byte("idCard")) || !bytes.Contains(w.Body.Bytes(), []byte("138****8001")) {
		t.Fatalf("bills leak sensitive fields: %s", w.Body.String())
	}

	w = doJSON(t, r, http.MethodPost, "/api/mp/pay/orders", tokenA, map[string]string{"tenantBillId": billID})
	var order struct{ OutTradeNo string }
	_ = json.Unmarshal(w.Body.Bytes(), &order)
	if w.Code != http.StatusOK || order.OutTradeNo == "" {
		t.Fatalf("create order: %d %s", w.Code, w.Body.String())
	}

	notify := map[string]interface{}{"outTradeNo": order.OutTradeNo, "success": true, "transactionId": "tx1"}
	if w := doJSON(t, r, http.MethodPost, "/api/mp/pay/notify", "", notify); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous notify: expected 401, got %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/mp/pay/notify", tokenB, notify); w.Code != http.StatusForbidden {
		t.Fatalf("notify for another user's order: expected 403, got %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/mp/pay/notify", tokenA, notify); w.Code != http.StatusOK {
		t.Fatalf("owner notify: %d %s", w.Code, w.Body.String())
	}
	if got := listBills(t, r)[billID]; got.Status != "已缴" || got.AmountPaid != 1000 {
		t.Fatalf("payment not applied: %+v", got)
	}
}

func billFor(t *testing.T, r *gin.Engine, room, month string) models.TenantRecord {
	t.Helper()
	for _, rec := range listBills(t, r) {
		if rec.RoomNumber == room && rec.Date == month {
			return rec
		}
	}
	t.Fatalf("no bill for room %s in %s", room, month)
	return models.TenantRecord{}
}

func TestQuarterlyTenantMonthlyBilling(t *testing.T) {
	r, _ := newTestServer(t)
	w := doJSON(t, r, http.MethodPost, "/api/tenants", testAdminToken, map[string]interface{}{
		"roomNumber": "501", "name": "赵六", "rentAmount": 1000, "rentCycle": "季度", "checkInDate": "2026-01-10",
		"waterReading": 100, "electricityReading": 1000, "date": "2026-01",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if got := billFor(t, r, "501", "2026-01"); got.RentDue != 3000 || got.TotalAmount != 3000 {
		t.Fatalf("Jan: want rentDue 3000 total 3000, got %+v", got)
	}

	for _, month := range []string{"2026-02", "2026-03", "2026-04"} {
		w := doJSON(t, r, http.MethodPost, "/api/bills/generate", testAdminToken, map[string]string{"month": month})
		if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"501"`)) {
			t.Fatalf("generate %s: %d %s", month, w.Code, w.Body.String())
		}
	}
	// Generating again is a no-op.
	w = doJSON(t, r, http.MethodPost, "/api/bills/generate", testAdminToken, map[string]string{"month": "2026-04"})
	if !bytes.Contains(w.Body.Bytes(), []byte(`"created":[]`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"skipped":1`)) {
		t.Fatalf("regenerate should skip: %s", w.Body.String())
	}

	feb := billFor(t, r, "501", "2026-02")
	if feb.RentDue != 0 || feb.TotalAmount != 0 || feb.WaterReading != 100 || feb.Name != "赵六" {
		t.Fatalf("Feb: want no rent, carried readings, got %+v", feb)
	}
	if apr := billFor(t, r, "501", "2026-04"); apr.RentDue != 3000 {
		t.Fatalf("Apr: want rentDue 3000 (next quarter), got %v", apr.RentDue)
	}

	// Meter reading for Feb: client sends bills 0 so the server computes utilities.
	w = doJSON(t, r, http.MethodPut, "/api/tenants/"+feb.ID, testAdminToken, map[string]interface{}{
		"waterReading": 104, "electricityReading": 1100, "waterBill": 0, "electricityBill": 0, "extraFee": 20,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("meter update: %d %s", w.Code, w.Body.String())
	}
	feb = billFor(t, r, "501", "2026-02")
	// water 4 × 5.5 = 22, electricity 100 × 1.2 = 120, extra 20, rent 0
	if feb.WaterBill != 22 || feb.ElectricityBill != 120 || feb.ExtraFee != 20 || feb.TotalAmount != 162 {
		t.Fatalf("Feb after meter reading: %+v", feb)
	}

	// Editing contact info keeps money fields untouched.
	doJSON(t, r, http.MethodPut, "/api/tenants/"+feb.ID, testAdminToken, map[string]interface{}{"phone": "13700000000"})
	if got := billFor(t, r, "501", "2026-02"); got.TotalAmount != 162 || got.Phone != "13700000000" {
		t.Fatalf("contact edit changed totals: %+v", got)
	}

	// Switching to monthly rent re-derives this bill's rent.
	doJSON(t, r, http.MethodPut, "/api/tenants/"+feb.ID, testAdminToken, map[string]interface{}{"rentCycle": "月度"})
	if got := billFor(t, r, "501", "2026-02"); got.RentDue != 1000 || got.TotalAmount != 1162 {
		t.Fatalf("after cycle change: %+v", got)
	}
}

func TestQuarterlyCollectionAcrossMonths(t *testing.T) {
	r, _ := newTestServer(t)
	doJSON(t, r, http.MethodPost, "/api/tenants", testAdminToken, map[string]interface{}{
		"roomNumber": "601", "name": "孙七", "rentAmount": 1000, "rentCycle": "季度", "checkInDate": "2026-01-05",
		"waterReading": 0, "electricityReading": 0, "date": "2026-01",
	})
	// Meters are read every month; each reading adds 10 kWh (12 yuan) of electricity.
	for i, month := range []string{"2026-02", "2026-03", "2026-04"} {
		doJSON(t, r, http.MethodPost, "/api/bills/generate", testAdminToken, map[string]string{"month": month})
		bill := billFor(t, r, "601", month)
		w := doJSON(t, r, http.MethodPut, "/api/tenants/"+bill.ID, testAdminToken, map[string]interface{}{
			"waterReading": 0, "electricityReading": float64(10 * (i + 1)), "waterBill": 0, "electricityBill": 0,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("meter %s: %d %s", month, w.Code, w.Body.String())
		}
	}

	wantDue := map[string]string{"2026-01": "2026-01", "2026-02": "2026-04", "2026-03": "2026-04", "2026-04": "2026-04"}
	for month, due := range wantDue {
		if got := billFor(t, r, "601", month); got.DueMonth != due {
			t.Fatalf("%s: dueMonth %s, want %s", month, got.DueMonth, due)
		}
	}

	// Pay January (rent 3000), then the April collection: Feb+Mar utilities 24 + April rent 3000 + April utilities 12.
	pay := func(amount float64) string {
		w := doJSON(t, r, http.MethodPost, "/api/rooms/601/payments", testAdminToken, map[string]float64{"amount": amount})
		if w.Code != http.StatusOK {
			t.Fatalf("room payment: %d %s", w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	pay(3000)
	if got := billFor(t, r, "601", "2026-01"); got.Status != "已缴" {
		t.Fatalf("January not settled: %+v", got)
	}
	if body := pay(3036 + 50); !strings.Contains(body, `"unapplied":50`) {
		t.Fatalf("expected 50 unapplied: %s", body)
	}
	for _, month := range []string{"2026-02", "2026-03", "2026-04"} {
		if got := billFor(t, r, "601", month); got.Status != "已缴" {
			t.Fatalf("%s not settled by the April collection: %+v", month, got)
		}
	}
	if w := doJSON(t, r, http.MethodPost, "/api/rooms/601/payments", testAdminToken, map[string]float64{"amount": 1}); w.Code != http.StatusNotFound {
		t.Fatalf("payment with nothing owed: expected 404, got %d", w.Code)
	}
}
