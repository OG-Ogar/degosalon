package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"barber-booking/internal/googleauth"
	"barber-booking/models"
	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

type googleUserFixture struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
}

// installGoogleProvider points the handler at a local token/userinfo server so
// the tests never contact Google. It restores whatever provider was configured
// before the test, keeping test order irrelevant.
func installGoogleProvider(t *testing.T, user googleUserFixture) {
	t.Helper()
	previous := googleSignIn
	t.Cleanup(func() { googleSignIn = previous })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.Method != http.MethodPost {
				http.Error(w, "method", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access-token","token_type":"Bearer"}`))
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer test-access-token" {
				http.Error(w, "authorization", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	provider, err := googleauth.NewProvider(googleauth.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "https://salon.example.com/auth/google/callback",
		Endpoints: googleauth.Endpoints{
			AuthURL:     server.URL + "/authorize",
			TokenURL:    server.URL + "/token",
			UserInfoURL: server.URL + "/userinfo",
		},
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	googleSignIn = provider
}

func googleStart(t *testing.T, role, next string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/auth/google/login?role=" + url.QueryEscape(role)
	if next != "" {
		target += "&next=" + url.QueryEscape(next)
	}
	response := httptest.NewRecorder()
	GoogleAuthStartHandler(response, httptest.NewRequest(http.MethodGet, target, nil))
	return response
}

func googleCallback(t *testing.T, state, cookieValue string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/auth/google/callback?state=" + url.QueryEscape(state) + "&code=test-code"
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.AddCookie(&http.Cookie{Name: googleStateCookie, Value: cookieValue})
	response := httptest.NewRecorder()
	GoogleAuthCallbackHandler(response, request)
	return response
}

// googleRoundTrip runs the start and callback handlers and returns the final
// response together with the nonce and signed cookie from the first step.
func googleRoundTrip(t *testing.T, role, next string) (*httptest.ResponseRecorder, string, string) {
	t.Helper()
	start := googleStart(t, role, next)
	if start.Code != http.StatusSeeOther {
		t.Fatalf("start status %d: %s", start.Code, start.Body.String())
	}
	location, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	cookie := responseCookie(start, googleStateCookie)
	if cookie == nil {
		t.Fatal("start did not set the state cookie")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/auth/google" {
		t.Fatalf("state cookie attributes: %+v", cookie)
	}
	return googleCallback(t, location.Query().Get("state"), cookie.Value), location.Query().Get("state"), cookie.Value
}

func responseCookie(response *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestGoogleAuthStartBuildsAuthorizationRedirect(t *testing.T) {
	installGoogleProvider(t, googleUserFixture{Subject: "s", Email: "ada@example.com", EmailVerified: true, Name: "Ada"})
	response := googleStart(t, "customer", "/customer/bookings")
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := location.Query()
	if query.Get("client_id") != "test-client" || query.Get("state") == "" || query.Get("code_challenge") == "" {
		t.Fatalf("authorization redirect: %s", location)
	}
	if query.Get("code_challenge_method") != "S256" {
		t.Fatal("authorization redirect did not request S256 PKCE")
	}
	if cookie := responseCookie(response, googleStateCookie); cookie == nil {
		t.Fatal("state cookie missing")
	}
}

func TestGoogleAuthStartRejectsUnknownRole(t *testing.T) {
	installGoogleProvider(t, googleUserFixture{Email: "ada@example.com", EmailVerified: true})
	response := googleStart(t, "admin", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status %d", response.Code)
	}
	if cookie := responseCookie(response, googleStateCookie); cookie != nil {
		t.Fatal("rejected start issued a state cookie")
	}
}

func TestGoogleAuthUnavailableWithoutConfiguration(t *testing.T) {
	previous := googleSignIn
	googleSignIn = nil
	t.Cleanup(func() { googleSignIn = previous })

	start := googleStart(t, "customer", "")
	if start.Code != http.StatusServiceUnavailable {
		t.Fatalf("start status %d", start.Code)
	}
	callback := httptest.NewRecorder()
	GoogleAuthCallbackHandler(callback, httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=x&code=y", nil))
	if callback.Code != http.StatusServiceUnavailable {
		t.Fatalf("callback status %d", callback.Code)
	}
}

func TestGoogleAuthCallbackCreatesCustomerAndStartsSession(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	installGoogleProvider(t, googleUserFixture{
		Subject: "google-subject", Email: "New.Customer@Example.com", EmailVerified: true, Name: "New Customer",
	})

	response, _, _ := googleRoundTrip(t, "customer", "/customer/bookings")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/customer/bookings" {
		t.Fatalf("callback status %d location %q: %s", response.Code, response.Header().Get("Location"), response.Body.String())
	}

	cookie := responseCookie(response, session.CookieName(session.RoleCustomer))
	if cookie == nil {
		t.Fatal("customer session cookie missing")
	}
	active, ok := session.Get(cookie.Value)
	if !ok || active.Role != session.RoleCustomer || active.UserID < 1 {
		t.Fatalf("session not stored: %+v %v", active, ok)
	}

	customer, err := storage.FindCustomerByEmailResult(context.Background(), "new.customer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if customer.FullName != "New Customer" {
		t.Fatalf("name = %q", customer.FullName)
	}
	if customer.PhoneNumber != "" {
		t.Fatalf("phone = %q, want empty until booking", customer.PhoneNumber)
	}
	if customer.PasswordHash == "" || security.VerifyPassword(customer.PasswordHash, "") {
		t.Fatal("Google-only account must have an unusable password")
	}
}

func TestGoogleAuthCallbackLinksExistingCustomer(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	existing, err := storage.CreateCustomer(models.Customer{
		FullName: "Existing Person", PhoneNumber: "08012345678", Email: "ada@example.com", PasswordHash: "existing-hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	installGoogleProvider(t, googleUserFixture{
		Subject: "google-subject", Email: "Ada@Example.com", EmailVerified: true, Name: "Ada From Google",
	})

	response, _, _ := googleRoundTrip(t, "customer", "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/customer/dashboard" {
		t.Fatalf("callback status %d location %q", response.Code, response.Header().Get("Location"))
	}
	if count := len(storage.ListCustomers()); count != 1 {
		t.Fatalf("customer count = %d, want 1", count)
	}
	linked, err := storage.FindCustomerByIDResult(context.Background(), existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if linked.FullName != "Existing Person" || linked.PasswordHash != "existing-hash" {
		t.Fatalf("existing account changed: %+v", linked)
	}
	if cookie := responseCookie(response, session.CookieName(session.RoleCustomer)); cookie == nil {
		t.Fatal("customer session cookie missing")
	}
}

func TestGoogleAuthCallbackCreatesBarberAndStartsSession(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	installGoogleProvider(t, googleUserFixture{
		Subject: "google-subject", Email: "barber@example.com", EmailVerified: true, Name: "Barber Ada",
	})

	response, _, _ := googleRoundTrip(t, "barber", "/barber/dashboard")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/barber/dashboard" {
		t.Fatalf("callback status %d location %q: %s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	barber, err := storage.FindBarberByEmailResult(context.Background(), "barber@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if barber.FullName != "Barber Ada" || !barber.IsActive() || barber.ShopName == "" {
		t.Fatalf("provisioned barber: %+v", barber)
	}
	if cookie := responseCookie(response, session.CookieName(session.RoleBarber)); cookie == nil {
		t.Fatal("barber session cookie missing")
	}
}

func TestGoogleAuthCallbackRejectsSuspendedBarber(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	if _, err := storage.CreateBarber(models.Barber{
		FullName: "Suspended", Email: "barber@example.com", PasswordHash: "hash", Status: models.BarberSuspended,
	}); err != nil {
		t.Fatal(err)
	}
	installGoogleProvider(t, googleUserFixture{
		Subject: "google-subject", Email: "barber@example.com", EmailVerified: true, Name: "Barber Ada",
	})

	response, _, _ := googleRoundTrip(t, "barber", "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", response.Code)
	}
	if cookie := responseCookie(response, session.CookieName(session.RoleBarber)); cookie != nil {
		t.Fatal("suspended barber received a session")
	}
}

func TestGoogleAuthCallbackRejectsUnverifiedEmail(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	installGoogleProvider(t, googleUserFixture{
		Subject: "google-subject", Email: "unverified@example.com", EmailVerified: false, Name: "Unverified",
	})

	response, _, _ := googleRoundTrip(t, "customer", "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", response.Code)
	}
	if len(storage.ListCustomers()) != 0 {
		t.Fatal("unverified email provisioned an account")
	}
	if cookie := responseCookie(response, session.CookieName(session.RoleCustomer)); cookie != nil {
		t.Fatal("unverified email received a session")
	}
}

func TestGoogleAuthCallbackRejectsMismatchedState(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	installGoogleProvider(t, googleUserFixture{Email: "ada@example.com", EmailVerified: true, Name: "Ada"})

	start := googleStart(t, "customer", "")
	cookie := responseCookie(start, googleStateCookie)
	if cookie == nil {
		t.Fatal("state cookie missing")
	}
	response := googleCallback(t, "not-the-nonce", cookie.Value)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", response.Code)
	}
	if cookie := responseCookie(response, session.CookieName(session.RoleCustomer)); cookie != nil {
		t.Fatal("mismatched state received a session")
	}
	if len(storage.ListCustomers()) != 0 {
		t.Fatal("mismatched state provisioned an account")
	}
}

func TestGoogleAuthCallbackRequiresStateCookie(t *testing.T) {
	installGoogleProvider(t, googleUserFixture{Email: "ada@example.com", EmailVerified: true, Name: "Ada"})
	response := googleCallback(t, "nonce", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", response.Code)
	}
}

func TestGoogleAuthCallbackDropsUnsafeReturnDestination(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	installGoogleProvider(t, googleUserFixture{
		Subject: "google-subject", Email: "ada@example.com", EmailVerified: true, Name: "Ada",
	})

	response, _, _ := googleRoundTrip(t, "customer", "https://evil.example/steal")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/customer/dashboard" {
		t.Fatalf("unsafe next reached %q", response.Header().Get("Location"))
	}
}

func TestGoogleAuthRejectsPostRequests(t *testing.T) {
	installGoogleProvider(t, googleUserFixture{Email: "ada@example.com", EmailVerified: true})
	response := httptest.NewRecorder()
	GoogleAuthStartHandler(response, httptest.NewRequest(http.MethodPost, "/auth/google/login?role=customer", strings.NewReader("")))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", response.Code)
	}
}
