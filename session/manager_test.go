package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionCookiesIsolationAndRevocation(t *testing.T) {
	ConfigureDatabase(nil)
	t.Cleanup(func() { sessions = map[string]Session{} })
	t.Setenv("SECURE_COOKIES", "1")
	w := httptest.NewRecorder()
	customer, err := StartContext(context.Background(), w, RoleCustomer, 1)
	if err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != int(Lifetime.Seconds()) || cookie.Path != "/" {
		t.Fatalf("cookie contract: %+v", cookie)
	}
	request := httptest.NewRequest(http.MethodGet, "/customer/dashboard", nil)
	request.AddCookie(cookie)
	if got, err := FromRequestContext(request, RoleCustomer); err != nil || got.CSRFToken != customer.CSRFToken {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	request.AddCookie(&http.Cookie{Name: CookieName(RoleBarber), Value: customer.ID})
	if _, err := FromRequestContext(request, RoleBarber); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-role cookie: %v", err)
	}
	other, err := New(RoleCustomer, 2)
	if err != nil {
		t.Fatal(err)
	}
	barber, err := New(RoleBarber, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteForUserContext(context.Background(), RoleCustomer, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := Get(customer.ID); ok {
		t.Fatal("revoked session usable")
	}
	for _, id := range []string{other.ID, barber.ID} {
		if _, ok := Get(id); !ok {
			t.Fatal("revoked another account or role")
		}
	}
	sessions[digest(other.ID)] = Session{ID: other.ID, ExpiresAt: time.Now().Add(-time.Second)}
	if _, err := GetContext(context.Background(), other.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session: %v", err)
	}
	for _, id := range []string{"", "bad", string(make([]byte, 64))} {
		if _, err := GetContext(context.Background(), id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("invalid token: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewContext(ctx, RoleCustomer, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled creation: %v", err)
	}
	for _, identity := range []struct {
		role Role
		id   int
	}{{RoleCustomer, 0}, {RoleBarber, -1}, {RoleAdmin, 1}, {"unknown", 1}} {
		if _, err := New(identity.role, identity.id); err == nil {
			t.Fatalf("invalid identity: %+v", identity)
		}
	}
}

func TestProductionForcesSecureCookies(t *testing.T) {
	for _, render := range []bool{false, true} {
		t.Setenv("APP_ENV", "production")
		t.Setenv("RENDER", "")
		if render {
			t.Setenv("APP_ENV", "development")
			t.Setenv("RENDER", "true")
		}
		t.Setenv("SECURE_COOKIES", "0")
		w := httptest.NewRecorder()
		s, err := StartContext(context.Background(), w, RoleCustomer, 22)
		if err != nil {
			t.Fatal(err)
		}
		Delete(s.ID)
		if !w.Result().Cookies()[0].Secure {
			t.Fatal("production session cookie was insecure")
		}
		request := httptest.NewRequest(http.MethodPost, "/logout", nil)
		request.AddCookie(w.Result().Cookies()[0])
		cleared := httptest.NewRecorder()
		if err := ClearContext(cleared, request, RoleCustomer); err != nil {
			t.Fatal(err)
		}
		if !cleared.Result().Cookies()[0].Secure {
			t.Fatal("production logout cookie was insecure")
		}
	}
}
