package googleauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testProvider(t *testing.T, handler http.Handler) (*Provider, *httptest.Server) {
	t.Helper()
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	config := Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://salon.example.com/auth/google/callback",
		Endpoints: Endpoints{
			AuthURL:     server.URL + "/authorize",
			TokenURL:    server.URL + "/token",
			UserInfoURL: server.URL + "/userinfo",
		},
	}
	provider, err := NewProvider(config, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return provider, server
}

func TestGoogleAuthStateRoundTripIsSignedAndBounded(t *testing.T) {
	provider, _ := testProvider(t, nil)
	state, err := provider.NewState("customer", "/customer/bookings")
	if err != nil {
		t.Fatal(err)
	}
	if state.Nonce == "" || state.Verifier == "" || state.CookieValue == "" {
		t.Fatalf("incomplete state: %+v", state)
	}
	if len(state.Verifier) < 43 || len(state.Verifier) > 128 {
		t.Fatalf("PKCE verifier length %d outside RFC 7636 range", len(state.Verifier))
	}

	payload, err := provider.ReadState(state.CookieValue)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Nonce != state.Nonce || payload.Verifier != state.Verifier || payload.Role != "customer" || payload.Next != "/customer/bookings" {
		t.Fatalf("state payload changed: %+v", payload)
	}

	tampered := state.CookieValue[:len(state.CookieValue)-1] + "A"
	if tampered == state.CookieValue {
		tampered = state.CookieValue[:len(state.CookieValue)-1] + "B"
	}
	if _, err := provider.ReadState(tampered); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("tampered state accepted: %v", err)
	}

	// Anyone can edit the clear-text part of the cookie. Changing the role
	// must invalidate the signature.
	body, signature, _ := strings.Cut(state.CookieValue, ".")
	_ = body
	if _, err := provider.ReadState("forged." + signature); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("forged body accepted: %v", err)
	}
}

func TestGoogleAuthStateExpires(t *testing.T) {
	provider, _ := testProvider(t, nil)
	state, err := provider.NewState("barber", "/barber/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	if _, err := provider.ReadState(state.CookieValue); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expired state accepted: %v", err)
	}

	fresh, err := provider.NewState("barber", "")
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Now().Add(-2 * time.Minute) }
	if _, err := provider.ReadState(fresh.CookieValue); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("future-issued state accepted: %v", err)
	}
}

func TestGoogleAuthAuthCodeURLUsesPKCE(t *testing.T) {
	provider, _ := testProvider(t, nil)
	state, err := provider.NewState("customer", "")
	if err != nil {
		t.Fatal(err)
	}
	raw := provider.AuthCodeURL(state.Nonce, state.Verifier)
	address, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := address.Query()
	if query.Get("client_id") != "client-id" || query.Get("response_type") != "code" {
		t.Fatalf("authorization query: %v", query)
	}
	if query.Get("state") != state.Nonce {
		t.Fatal("authorization URL did not carry the nonce")
	}
	if query.Get("code_challenge") != Challenge(state.Verifier) || query.Get("code_challenge_method") != "S256" {
		t.Fatal("authorization URL did not use S256 PKCE")
	}
	if !strings.Contains(query.Get("scope"), "email") {
		t.Fatalf("scope missing email: %s", query.Get("scope"))
	}
	if query.Get("redirect_uri") != "https://salon.example.com/auth/google/callback" {
		t.Fatalf("redirect URI: %s", query.Get("redirect_uri"))
	}
}

func TestGoogleAuthExchangeReadsVerifiedUserInfo(t *testing.T) {
	var tokenForm url.Values
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				http.Error(w, "bad token request", http.StatusBadRequest)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			tokenForm = r.PostForm
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"access-token","token_type":"Bearer"}`)
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"sub":"google-subject","email":"Ada@Example.com","email_verified":true,"name":"Ada Lovelace","given_name":"Ada"}`)
		default:
			http.NotFound(w, r)
		}
	})
	provider, _ := testProvider(t, handler)
	state, err := provider.NewState("customer", "")
	if err != nil {
		t.Fatal(err)
	}

	user, err := provider.Exchange(context.Background(), "authorization-code", state.Verifier)
	if err != nil {
		t.Fatal(err)
	}
	if user.Subject != "google-subject" || user.Email != "Ada@Example.com" || !user.EmailVerified || user.Name != "Ada Lovelace" {
		t.Fatalf("unexpected user: %+v", user)
	}
	for field, want := range map[string]string{
		"code":          "authorization-code",
		"client_id":     "client-id",
		"client_secret": "client-secret",
		"redirect_uri":  "https://salon.example.com/auth/google/callback",
		"grant_type":    "authorization_code",
		"code_verifier": state.Verifier,
	} {
		if got := tokenForm.Get(field); got != want {
			t.Fatalf("token form %s = %q, want %q", field, got, want)
		}
	}
}

func TestGoogleAuthRejectsUnverifiedEmailAddress(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			fmt.Fprint(w, `{"access_token":"access-token"}`)
		case "/userinfo":
			fmt.Fprint(w, `{"sub":"google-subject","email":"unverified@example.com","email_verified":false}`)
		default:
			http.NotFound(w, r)
		}
	})
	provider, _ := testProvider(t, handler)
	state, err := provider.NewState("customer", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Exchange(context.Background(), "code", state.Verifier); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("unverified email accepted: %v", err)
	}
}

func TestGoogleAuthExchangeFailureDoesNotLeakSecrets(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant","error_description":"Bad Request"}`)
	})
	provider, _ := testProvider(t, handler)
	state, err := provider.NewState("customer", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Exchange(context.Background(), "code", state.Verifier)
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("expected provider rejection, got %v", err)
	}
	if strings.Contains(err.Error(), "client-secret") {
		t.Fatal("token exchange error leaked the client secret")
	}
}

func TestGoogleAuthProviderRejectsIncompleteConfiguration(t *testing.T) {
	for _, config := range []Config{
		{ClientSecret: "secret", RedirectURL: "https://example.com/cb", Endpoints: DefaultEndpoints()},
		{ClientID: "id", RedirectURL: "https://example.com/cb", Endpoints: DefaultEndpoints()},
		{ClientID: "id", ClientSecret: "secret", RedirectURL: "/relative", Endpoints: DefaultEndpoints()},
		{ClientID: "id", ClientSecret: "secret", RedirectURL: "https://example.com/cb", Endpoints: Endpoints{}},
	} {
		if _, err := NewProvider(config, nil); err == nil {
			t.Fatalf("incomplete configuration accepted: %+v", config)
		}
	}
}

func TestGoogleAuthConfigFromEnvironment(t *testing.T) {
	clear := func(t *testing.T) {
		t.Helper()
		for _, name := range []string{"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL", "SITE_URL", "RENDER_EXTERNAL_URL"} {
			t.Setenv(name, "")
		}
	}

	t.Run("disabled without credentials", func(t *testing.T) {
		clear(t)
		if _, enabled, err := ConfigFromEnvironment(); err != nil || enabled {
			t.Fatalf("enabled=%v err=%v", enabled, err)
		}
	})

	t.Run("half-set credentials fail closed", func(t *testing.T) {
		clear(t)
		t.Setenv("GOOGLE_CLIENT_ID", "id")
		if _, enabled, err := ConfigFromEnvironment(); err == nil || enabled {
			t.Fatalf("half-set credentials accepted: enabled=%v err=%v", enabled, err)
		}
	})

	t.Run("redirect derives from site origin", func(t *testing.T) {
		clear(t)
		t.Setenv("GOOGLE_CLIENT_ID", "id")
		t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
		t.Setenv("SITE_URL", "https://salon.example.com/")
		config, enabled, err := ConfigFromEnvironment()
		if err != nil || !enabled {
			t.Fatalf("enabled=%v err=%v", enabled, err)
		}
		if config.RedirectURL != "https://salon.example.com/auth/google/callback" {
			t.Fatalf("redirect: %s", config.RedirectURL)
		}
	})

	t.Run("site origin with a path is rejected", func(t *testing.T) {
		clear(t)
		t.Setenv("GOOGLE_CLIENT_ID", "id")
		t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
		t.Setenv("SITE_URL", "https://salon.example.com/app")
		if _, enabled, err := ConfigFromEnvironment(); err == nil || enabled {
			t.Fatalf("site URL with path accepted: enabled=%v err=%v", enabled, err)
		}
	})

	t.Run("explicit redirect wins", func(t *testing.T) {
		clear(t)
		t.Setenv("GOOGLE_CLIENT_ID", "id")
		t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
		t.Setenv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback")
		config, enabled, err := ConfigFromEnvironment()
		if err != nil || !enabled || config.RedirectURL != "http://localhost:8080/auth/google/callback" {
			t.Fatalf("explicit redirect: %+v enabled=%v err=%v", config, enabled, err)
		}
	})
}
