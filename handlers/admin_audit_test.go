package handlers

import (
	"barber-booking/internal/testdb"
	"barber-booking/storage"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAdminAuditCapturesHandlerResultsAndCanceledRequests(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	t.Setenv("ADMIN_USERNAME", "auditoperator")
	t.Setenv("ADMIN_PASSWORD", "browser-test-password")
	if _, err := InitAdmin(); err != nil {
		t.Fatal(err)
	}
	if err := LoadTemplates("../templates"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("POST", "/admin/barbers/42/suspend", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	withAdminAudit(response, request, func(w http.ResponseWriter, r *http.Request) { cancel(); w.WriteHeader(403) })
	events, err := storage.ListAdminAudit(context.Background(), 0, 10)
	if err != nil || len(events) != 2 {
		t.Fatalf("audit: %v %v", events, err)
	}
	if events[0].HTTPStatus != 403 || events[0].Actor != "auditoperator" || events[0].Action != "barber.suspend" || events[0].TargetID != 42 || events[0].RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("wrong audit metadata: %v", events)
	}
	withAdminAudit(httptest.NewRecorder(), httptest.NewRequest("GET", "/admin/barbers/42/delete", nil), func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	events, _ = storage.ListAdminAudit(context.Background(), 0, 10)
	if len(events) != 2 {
		t.Fatal("read-only action audited as mutation")
	}
}
func TestAdminAuditCreationRecordsNewAccountID(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	t.Setenv("ADMIN_USERNAME", "auditoperator")
	t.Setenv("ADMIN_PASSWORD", "browser-test-password")
	if _, err := InitAdmin(); err != nil {
		t.Fatal(err)
	}
	if err := LoadTemplates("../templates"); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"csrf_token": {"auditcsrf"}, "fullname": {"Audit Barber"}, "phone": {"08012345678"}, "email": {"audit@example.test"}, "shopname": {"Audit Shop"}, "shopaddress": {"12 Example Street"}, "state": {"Lagos"}, "local_area": {"Ikeja"}, "password": {"browser-pass-123"}}
	request := httptest.NewRequest("POST", "/admin/barbers/new", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	withAdminAudit(response, request, func(w http.ResponseWriter, r *http.Request) {
		NewBarberHandler(w, r, sessionData{CSRFToken: "auditcsrf"})
	})
	if response.Code != 303 {
		t.Fatalf("create failed: %d %s", response.Code, response.Body.String())
	}
	events, err := storage.ListAdminAudit(context.Background(), 0, 10)
	if err != nil || len(events) != 2 {
		t.Fatalf("audit list: %v %v", events, err)
	}
	if events[0].TargetID < 1 || events[0].HTTPStatus != 303 || events[1].TargetID != 0 {
		t.Fatalf("new account attribution missing: %v", events)
	}
	if _, err := storage.FindBarberByIDContext(context.Background(), events[0].TargetID); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAdminAuditFailurePreventsMutation(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", address)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.CloseDatabase() })
	t.Setenv("ADMIN_USERNAME", "auditoperator")
	t.Setenv("ADMIN_PASSWORD", "browser-test-password")
	if _, err := InitAdmin(); err != nil {
		t.Fatal(err)
	}
	if err := LoadTemplates("../templates"); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec("alter table degosalon_internal.admin_audit_events rename to unavailable_audit_events"); err != nil {
		t.Fatal(err)
	}
	called := false
	response := httptest.NewRecorder()
	withAdminAudit(response, httptest.NewRequest("POST", "/admin/barbers/42/delete", nil), func(http.ResponseWriter, *http.Request) { called = true })
	if called || response.Code != 500 {
		t.Fatalf("unaudited mutation ran: %t %d", called, response.Code)
	}
}
