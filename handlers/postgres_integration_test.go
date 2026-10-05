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
