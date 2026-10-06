package handlers

import (
	"barber-booking/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestForgotPasswordThrottleAndRecoveryHeaders(t *testing.T) {
	configureRecoveryMail(t)
	storage.Reset()
	t.Cleanup(storage.Reset)
	for i := 0; i < 4; i++ {
		w := httptest.NewRecorder()
		r := authFormRequest("/customer/forgot-password", url.Values{"email": {"unknown_throttle@example.com"}})
		CustomerForgotPasswordHandler(w, r)
		want := 200
		if i == 3 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
		if want == 429 && (w.Header().Get("Retry-After") == "" || w.Header().Get("Cache-Control") != "no-store") {
			t.Fatal("throttle missing retry headers")
		}
	}
	w := httptest.NewRecorder()
	CustomerForgotPasswordHandler(w, httptest.NewRequest(http.MethodGet, "/customer/forgot-password", nil))
	if w.Code != 200 {
		t.Fatal("throttle blocked recovery form")
	}
	w = httptest.NewRecorder()
	CustomerForgotPasswordHandler(w, authFormRequest("/customer/forgot-password", url.Values{"email": {"other_throttle@example.com"}}))
	if w.Code != 200 {
		t.Fatal("account throttle blocked another account")
	}
	w = httptest.NewRecorder()
	CustomerLoginHandler(w, authFormRequest("/customer/login", url.Values{"email": {strings.Repeat("x", 70000)}, "password": {"password"}}))
	if w.Code != 413 {
		t.Fatalf("large login form: %d", w.Code)
	}
}
func TestForwardedAddressRequiresTrustedProxy(t *testing.T) {
	for _, test := range []struct{ trusted, peer, forwarded, want string }{
		{"", "198.51.100.1:1234", "203.0.113.2", "198.51.100.1"},
		{"10.0.0.0/8", "10.0.0.2:1234", "6.6.6.6, 203.0.113.2", "203.0.113.2"},
		{"10.0.0.0/8", "10.0.0.2:1234", "6.6.6.6, 203.0.113.2, 10.0.0.3", "203.0.113.2"},
		{"10.0.0.0/8", "198.51.100.1:1234", "203.0.113.2", "198.51.100.1"},
		{"10.0.0.0/8", "10.0.0.2:1234", "invalid", "10.0.0.2"},
	} {
		t.Setenv("TRUSTED_PROXY_CIDRS", test.trusted)
		r := httptest.NewRequest(http.MethodPost, "/login", nil)
		r.RemoteAddr = test.peer
		r.Header.Set("X-Forwarded-For", test.forwarded)
		if got := authClientAddress(r); got != test.want {
			t.Fatalf("client address %s, want %s", got, test.want)
		}
	}
}
