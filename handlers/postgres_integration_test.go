package handlers

import (
	"barber-booking/reset"
	"barber-booking/security"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"barber-booking/internal/testdb"
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
)

func TestPostgresAuthenticationSurvivesQueryFailure(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", databaseURL)
	if e := storage.InitDatabase(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { storage.CloseDatabase() })
	connection, e := sql.Open("pgx", databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { connection.Close() })
	customer, e := storage.CreateCustomer(models.Customer{FullName: "Customer", Email: "customer@example.com"})
	if e != nil {
		t.Fatal(e)
	}
	barber, e := storage.CreateBarber(models.Barber{FullName: "Barber", ShopName: "Shop", Email: "barber@example.com"})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		role  session.Role
		id    int
		table string
		login http.HandlerFunc
	}{
		{session.RoleCustomer, customer.ID, "customers", CustomerLoginHandler},
		{session.RoleBarber, barber.ID, "barbers", BarberLoginHandler},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			s, e := session.New(tc.role, tc.id)
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { session.Delete(s.ID) })
			request := httptest.NewRequest(http.MethodGet, "/"+string(tc.role)+"/dashboard", nil)
			request.AddCookie(&http.Cookie{Name: session.CookieName(tc.role), Value: s.ID})
			// Rename only a table in the disposable database to inject a real SQL error.
			if _, e := connection.Exec("alter table public." + tc.table + " rename to unavailable_" + tc.table); e != nil {
				t.Fatal(e)
			}
			response := httptest.NewRecorder()
			if checkAccountSession(tc.role, response, request) || response.Code != http.StatusInternalServerError {
				t.Fatalf("failed lookup response=%d", response.Code)
			}
			if _, ok := session.Get(s.ID); !ok || len(response.Result().Cookies()) != 0 {
				t.Fatal("database error cleared session")
			}
			login := httptest.NewRecorder()
			tc.login(login, authFormRequest("/login", url.Values{"email": {"account@example.com"}, "password": {"password"}}))
			if login.Code != http.StatusInternalServerError {
				t.Fatalf("failed SQL lookup reported credentials failure: %d", login.Code)
			}
			if _, e := connection.Exec("alter table public.unavailable_" + tc.table + " rename to " + tc.table); e != nil {
				t.Fatal(e)
			}
			if !checkAccountSession(tc.role, httptest.NewRecorder(), request) {
				t.Fatal("session unusable after database query recovers")
			}
		})
	}
}

func TestPostgresSessionOutagePreservesAuthentication(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", databaseURL)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.CloseDatabase() })
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	customer, err := storage.CreateCustomer(models.Customer{Email: "session_outage@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.New(session.RoleCustomer, customer.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/customer/dashboard", nil)
	request.AddCookie(&http.Cookie{Name: session.CookieName(session.RoleCustomer), Value: s.ID})
	if _, err := connection.Exec("alter table degosalon_internal.auth_sessions rename to unavailable_sessions"); err != nil {
		t.Fatal(err)
	}
	failed := httptest.NewRecorder()
	if checkAccountSession(session.RoleCustomer, failed, request) || failed.Code != 500 || len(failed.Result().Cookies()) != 0 {
		t.Fatalf("session outage response: %d", failed.Code)
	}
	logoutRequest := authFormRequest("/customer/logout", url.Values{"csrf_token": {"test"}})
	logoutRequest.AddCookie(&http.Cookie{Name: session.CookieName(session.RoleCustomer), Value: s.ID})
	s.CSRFToken = "test"
	logout := httptest.NewRecorder()
	CustomerLogoutHandler(logout, logoutRequest, s)
	if logout.Code != 500 || len(logout.Result().Cookies()) != 0 {
		t.Fatalf("failed logout: %d", logout.Code)
	}
	if _, err := connection.Exec("alter table degosalon_internal.unavailable_sessions rename to auth_sessions"); err != nil {
		t.Fatal(err)
	}
	if !checkAccountSession(session.RoleCustomer, httptest.NewRecorder(), request) {
		t.Fatal("session unusable after outage")
	}
	storage.CloseDatabase()
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	if !checkAccountSession(session.RoleCustomer, httptest.NewRecorder(), request) {
		t.Fatal("session lost on storage reconnect")
	}
	logout = httptest.NewRecorder()
	CustomerLogoutHandler(logout, logoutRequest, s)
	if logout.Code != 303 || len(logout.Result().Cookies()) != 1 {
		t.Fatalf("recovered logout: %d", logout.Code)
	}
	if _, ok := session.Get(s.ID); ok {
		t.Fatal("logout failed to revoke durable session")
	}
}

func TestPostgresPasswordRecoveryAfterFailedWrite(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", databaseURL)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.CloseDatabase() })
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	oldHash, err := security.HashPassword("old recovery password")
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{Email: "recovery_customer@example.com", PasswordHash: oldHash})
	if err != nil {
		t.Fatal(err)
	}
	barber, err := storage.CreateBarber(models.Barber{Email: "recovery_barber@example.com", PasswordHash: oldHash})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		role    session.Role
		id      int
		table   string
		handler http.HandlerFunc
	}{
		{session.RoleCustomer, customer.ID, "customers", CustomerResetPasswordHandler},
		{session.RoleBarber, barber.ID, "barbers", BarberResetPasswordHandler},
	} {
		t.Run(string(test.role), func(t *testing.T) {
			token, err := reset.NewContext(context.Background(), test.role, test.id)
			if err != nil {
				t.Fatal(err)
			}
			s, err := session.New(test.role, test.id)
			if err != nil {
				t.Fatal(err)
			}
			page := httptest.NewRecorder()
			test.handler(page, httptest.NewRequest(http.MethodGet, "/reset-password?token="+token, nil))
			if page.Code != 200 || page.Header().Get("Cache-Control") != "no-store" || page.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatalf("reset form headers: %d", page.Code)
			}
			// The token table outage must be an operational error, not an expired link.
			if _, err := connection.Exec("alter table degosalon_internal.password_resets rename to unavailable_resets"); err != nil {
				t.Fatal(err)
			}
			unavailable := httptest.NewRecorder()
			test.handler(unavailable, httptest.NewRequest(http.MethodGet, "/reset-password?token="+token, nil))
			if unavailable.Code != 500 {
				t.Fatalf("reset lookup outage: %d", unavailable.Code)
			}
			if _, err := connection.Exec("alter table degosalon_internal.unavailable_resets rename to password_resets"); err != nil {
				t.Fatal(err)
			}
			if _, err := connection.Exec("alter table public." + test.table + " rename to unavailable_" + test.table); err != nil {
				t.Fatal(err)
			}
			form := url.Values{"token": {token}, "password": {"new recovery password"}, "confirm_password": {"new recovery password"}}
			failed := httptest.NewRecorder()
			test.handler(failed, authFormRequest("/reset-password", form))
			if failed.Code != 500 {
				t.Fatalf("write outage: %d", failed.Code)
			}
			if _, err := reset.LookupContext(context.Background(), test.role, token); err != nil {
				t.Fatalf("failed write burned token: %v", err)
			}
			if _, err := session.GetContext(context.Background(), s.ID); err != nil {
				t.Fatalf("failed write revoked session: %v", err)
			}
			if _, err := connection.Exec("alter table public.unavailable_" + test.table + " rename to " + test.table); err != nil {
				t.Fatal(err)
			}
			success := httptest.NewRecorder()
			test.handler(success, authFormRequest("/reset-password", form))
			if success.Code != 303 {
				t.Fatalf("recovered reset: %d", success.Code)
			}
			var hash string
			if err := connection.QueryRow("select password_hash from public."+test.table+" where id=$1", test.id).Scan(&hash); err != nil || !security.VerifyPassword(hash, "new recovery password") || security.VerifyPassword(hash, "old recovery password") {
				t.Fatalf("password result: %v", err)
			}
			if _, err := session.GetContext(context.Background(), s.ID); !errors.Is(err, session.ErrNotFound) {
				t.Fatalf("old session remains: %v", err)
			}
			reused := httptest.NewRecorder()
			test.handler(reused, authFormRequest("/reset-password", form))
			if reused.Code != 400 {
				t.Fatalf("reused link: %d", reused.Code)
			}
		})
	}
}
