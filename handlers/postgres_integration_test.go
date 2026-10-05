package handlers

import (
	"database/sql"
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
