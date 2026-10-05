package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/reset"
	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

func TestAccountLookupFailurePreservesSessions(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer, err := storage.CreateCustomer(models.Customer{Email: "customer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	barber, err := storage.CreateBarber(models.Barber{Email: "barber@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		role session.Role
		id   int
	}{
		{session.RoleCustomer, customer.ID}, {session.RoleBarber, barber.ID},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			s, err := session.New(tc.role, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { session.Delete(s.ID) })
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			request := httptest.NewRequest(http.MethodGet, "/"+string(tc.role)+"/dashboard", nil).WithContext(ctx)
			request.AddCookie(&http.Cookie{Name: session.CookieName(tc.role), Value: s.ID})
			response := httptest.NewRecorder()
			if checkAccountSession(tc.role, response, request) {
				t.Fatal("failed lookup authorized request")
			}
			if response.Code != http.StatusInternalServerError || response.Header().Get("Location") != "" || len(response.Result().Cookies()) != 0 {
				t.Fatalf("failed lookup should neither redirect nor clear cookies: %d %+v", response.Code, response.Header())
			}
			if _, ok := session.Get(s.ID); !ok {
				t.Fatal("failed lookup deleted session")
			}
			recovered := httptest.NewRecorder()
			if !checkAccountSession(tc.role, recovered, request.WithContext(context.Background())) {
				t.Fatal("session unusable after lookup recovers")
			}
		})
	}
}

func checkAccountSession(role session.Role, response http.ResponseWriter, request *http.Request) bool {
	if role == session.RoleCustomer {
		_, _, ok := requireCustomer(response, request)
		return ok
	}
	_, _, ok := requireBarber(response, request)
	return ok
}

func TestMissingAndSuspendedAccountsStillClearSessions(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	barber, err := storage.CreateBarber(models.Barber{Email: "suspended@example.com", Status: models.BarberSuspended})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		role session.Role
		id   int
	}{
		{"missing customer", session.RoleCustomer, 99}, {"missing barber", session.RoleBarber, 99}, {"suspended barber", session.RoleBarber, barber.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := session.New(tc.role, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { session.Delete(s.ID) })
			request := httptest.NewRequest(http.MethodGet, "/"+string(tc.role)+"/dashboard", nil)
			request.AddCookie(&http.Cookie{Name: session.CookieName(tc.role), Value: s.ID})
			response := httptest.NewRecorder()
			if checkAccountSession(tc.role, response, request) {
				t.Fatal("unavailable account authorized")
			}
			destination, parseErr := url.Parse(response.Header().Get("Location"))
			if parseErr != nil || response.Code != http.StatusSeeOther || destination.Path != "/"+string(tc.role)+"/login" {
				t.Fatalf("missing login redirect: %d %+v", response.Code, response.Header())
			}
			if _, ok := session.Get(s.ID); ok {
				t.Fatal("unavailable account session retained")
			}
			cookies := response.Result().Cookies()
			if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
				t.Fatalf("missing expired cookie: %+v", cookies)
			}
		})
	}
}

func TestLoginLookupFailuresAreNotInvalidCredentials(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"customer", CustomerLoginHandler}, {"barber", BarberLoginHandler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			request := authFormRequest("/"+tc.name+"/login", url.Values{"email": {"account@example.com"}, "password": {"correct password"}}).WithContext(ctx)
			response := httptest.NewRecorder()
			tc.handler(response, request)
			if response.Code != http.StatusInternalServerError || len(response.Result().Cookies()) != 0 {
				t.Fatalf("lookup failure: %d %+v", response.Code, response.Header())
			}
			if strings.Contains(response.Body.String(), "Invalid email or password") || strings.Contains(response.Body.String(), "context canceled") {
				t.Fatal("lookup failure misreported or exposed internally")
			}
		})
	}
}

func TestLoginStillRejectsUnknownAndWrongPasswords(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	hash, err := security.HashPassword("correct password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateCustomer(models.Customer{Email: "account@example.com", PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateBarber(models.Barber{Email: "account@example.com", PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{CustomerLoginHandler, BarberLoginHandler} {
		for _, email := range []string{"missing@example.com", "account@example.com"} {
			response := httptest.NewRecorder()
			handler(response, authFormRequest("/login", url.Values{"email": {email}, "password": {"wrong password"}}))
			if response.Code != http.StatusUnauthorized || len(response.Result().Cookies()) != 0 {
				t.Fatalf("credentials rejection: %d", response.Code)
			}
			mustContain(t, "rejected login", response.Body.String(), "Invalid email or password.")
		}
	}
}

func TestForgotPasswordLookupFailureDoesNotClaimSuccessOrReplaceToken(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	reset.Clear()
	t.Cleanup(reset.Clear)
	for _, tc := range []struct {
		role    session.Role
		handler http.HandlerFunc
	}{
		{session.RoleCustomer, CustomerForgotPasswordHandler}, {session.RoleBarber, BarberForgotPasswordHandler},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			token, err := reset.New(tc.role, 1)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			response := httptest.NewRecorder()
			tc.handler(response, authFormRequest("/forgot-password", url.Values{"email": {"account@example.com"}}).WithContext(ctx))
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("lookup failure status: %d", response.Code)
			}
			if _, ok := reset.Lookup(tc.role, token); !ok {
				t.Fatal("failed lookup invalidated existing token")
			}
		})
	}
}

func authFormRequest(path string, values url.Values) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
