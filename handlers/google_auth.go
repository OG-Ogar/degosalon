package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"barber-booking/internal/googleauth"
	"barber-booking/internal/runtimeconfig"
	"barber-booking/models"
	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

const (
	// googleStateCookie carries the signed nonce, PKCE verifier, role and
	// return destination between the redirect and the callback.
	googleStateCookie = "google_sign_in_state"

	// googleStateLifetime mirrors the provider's own ten-minute bound.
	googleStateLifetime = 10 * time.Minute
)

// errGoogleBarberSuspended is the internal form of a rejection that is shown to
// the visitor as an ordinary 403 page. A suspended barber must not be able to
// obtain a session through Google any more than through the password form.
var errGoogleBarberSuspended = errors.New("barber account is suspended")

// googleSignIn is configured once at startup. A nil provider means the
// deployment has no Google client credentials, and both endpoints report that
// the feature is unavailable rather than failing with a 500.
var googleSignIn *googleauth.Provider

// ConfigureGoogleAuthFromEnvironment enables Google sign-in when credentials
// and a redirect URL are configured. A half-set configuration is reported to
// the caller and leaves the feature disabled; it never prevents the rest of the
// application from starting.
func ConfigureGoogleAuthFromEnvironment() (bool, error) {
	config, enabled, err := googleauth.ConfigFromEnvironment()
	if !enabled || err != nil {
		googleSignIn = nil
		return false, err
	}
	provider, err := googleauth.NewProvider(config, nil)
	if err != nil {
		googleSignIn = nil
		return false, err
	}
	googleSignIn = provider
	return true, nil
}

// GoogleAuthStartHandler begins the Google authorization-code flow.
//
// The role is part of the route because customers and barbers have separate
// account tables and separate session cookies; the signed state keeps a
// customer from completing a callback into the barber workspace.
func GoogleAuthStartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	provider := googleSignIn
	if provider == nil {
		journeyError(w, "Google sign-in is not available right now.", http.StatusServiceUnavailable)
		return
	}

	role, ok := googleRole(r.URL.Query().Get("role"))
	if !ok {
		journeyError(w, "Choose whether to sign in as a customer or a barber.", http.StatusBadRequest)
		return
	}
	next := safeGoogleNext(role, r.URL.Query().Get("next"))

	state, err := provider.NewState(string(role), next)
	if err != nil {
		serverError(w, "start Google sign-in", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     googleStateCookie,
		Value:    state.CookieValue,
		Path:     "/auth/google",
		MaxAge:   int(googleStateLifetime.Seconds()),
		HttpOnly: true,
		Secure:   googleCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, provider.AuthCodeURL(state.Nonce, state.Verifier), http.StatusSeeOther)
}

// GoogleAuthCallbackHandler completes the flow: it verifies the signed state,
// exchanges the authorization code, and either links the verified Google email
// to an existing account or provisions a new one before starting a session.
func GoogleAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	provider := googleSignIn
	if provider == nil {
		journeyError(w, "Google sign-in is not available right now.", http.StatusServiceUnavailable)
		return
	}

	// The state is single-use: clear the cookie before any check, so a
	// replayed callback cannot be completed twice.
	http.SetCookie(w, &http.Cookie{
		Name:     googleStateCookie,
		Value:    "",
		Path:     "/auth/google",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   googleCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})

	if !authBudget(w, r, "auth:google:peer:"+authClientAddress(r), 60, 15*time.Minute) {
		return
	}

	cookie, err := r.Cookie(googleStateCookie)
	if err != nil {
		googleSignInExpired(w)
		return
	}
	state, err := provider.ReadState(cookie.Value)
	if err != nil {
		googleSignInExpired(w)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state.Nonce)) != 1 {
		journeyError(w, "This Google sign-in could not be verified. Please try again.", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("error") != "" {
		journeyError(w, "Google sign-in was cancelled. Please try again.", http.StatusBadRequest)
		return
	}

	role := session.Role(state.Role)
	if role != session.RoleCustomer && role != session.RoleBarber {
		journeyError(w, "This Google sign-in could not be verified. Please try again.", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		journeyError(w, "Google sign-in did not return an authorization code. Please try again.", http.StatusBadRequest)
		return
	}

	user, err := provider.Exchange(r.Context(), code, state.Verifier)
	if errors.Is(err, googleauth.ErrEmailNotVerified) {
		journeyError(w, "That Google account's email address is not verified, so it cannot be used to sign in.", http.StatusForbidden)
		return
	}
	if err != nil {
		serverError(w, "complete Google sign-in", err)
		return
	}

	email := normalisedEmail(user.Email)
	if !validGoogleEmail(email) {
		journeyError(w, "That Google account did not return a usable email address.", http.StatusForbidden)
		return
	}

	accountID, err := googleAccount(r.Context(), role, user, email)
	if errors.Is(err, errGoogleBarberSuspended) {
		journeyError(w, "This barber account is suspended. Please contact the administrator.", http.StatusForbidden)
		return
	}
	if err != nil {
		serverError(w, "link Google account", err)
		return
	}

	if _, err := session.StartContext(r.Context(), w, role, accountID); err != nil {
		serverError(w, "start Google session", err)
		return
	}

	destination := "/customer/dashboard"
	switch role {
	case session.RoleBarber:
		destination = "/barber/dashboard"
	case session.RoleCustomer:
		if state.Next != "" {
			destination = state.Next
		}
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

func googleSignInExpired(w http.ResponseWriter) {
	journeyError(w, "This Google sign-in has expired. Please try again.", http.StatusBadRequest)
}

func googleRole(value string) (session.Role, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "customer":
		return session.RoleCustomer, true
	case "barber":
		return session.RoleBarber, true
	default:
		return "", false
	}
}

// safeGoogleNext limits the post-login destination to pages the role owns. The
// customer helper already rejects external and malformed URLs; barbers return
// to their dashboard and nothing else.
func safeGoogleNext(role session.Role, next string) string {
	if len(next) > 512 {
		return ""
	}
	if role == session.RoleCustomer {
		return safeCustomerNext(next)
	}
	if next == "/barber/dashboard" {
		return next
	}
	return ""
}

// googleAccount links a verified Google email to an existing customer or
// barber account. When no account exists it provisions one, because the
// approved sign-in design allows Google to create the account as well as link
// to one that already exists.
func googleAccount(ctx context.Context, role session.Role, user googleauth.User, email string) (int, error) {
	name := googleDisplayName(user, email)
	if role == session.RoleBarber {
		return googleBarber(ctx, name, email)
	}
	return googleCustomer(ctx, name, email)
}

func googleCustomer(ctx context.Context, name, email string) (int, error) {
	customer, err := storage.FindCustomerByEmailResult(ctx, email)
	if err == nil {
		return customer.ID, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return 0, err
	}

	hash, err := googlePlaceholderHash()
	if err != nil {
		return 0, err
	}
	created, err := storage.CreateCustomerContext(ctx, models.Customer{
		FullName: name,
		// Phone collection happens at booking confirmation when it is
		// actually needed, so Google sign-up does not demand it up front.
		PhoneNumber:  "",
		Email:        email,
		PasswordHash: hash,
	})
	if errors.Is(err, storage.ErrEmailTaken) {
		// A concurrent callback created the same account. Use it rather
		// than failing a sign-in that is otherwise valid.
		existing, lookupErr := storage.FindCustomerByEmailResult(ctx, email)
		if lookupErr != nil {
			return 0, lookupErr
		}
		return existing.ID, nil
	}
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func googleBarber(ctx context.Context, name, email string) (int, error) {
	barber, err := storage.FindBarberByEmailResult(ctx, email)
	if err == nil {
		if !barber.IsActive() {
			return 0, errGoogleBarberSuspended
		}
		return barber.ID, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return 0, err
	}

	hash, err := googlePlaceholderHash()
	if err != nil {
		return 0, err
	}
	created, err := storage.CreateBarberContext(ctx, models.Barber{
		FullName:     name,
		PhoneNumber:  "",
		Email:        email,
		PasswordHash: hash,
		// The shop details are completed from the profile page; the
		// account is usable immediately with a placeholder shop name.
		ShopName:    name + "'s shop",
		ShopAddress: "",
		State:       "",
		LocalArea:   "",
		Status:      models.BarberActive,
	})
	if errors.Is(err, storage.ErrEmailTaken) {
		existing, lookupErr := storage.FindBarberByEmailResult(ctx, email)
		if lookupErr != nil {
			return 0, lookupErr
		}
		if !existing.IsActive() {
			return 0, errGoogleBarberSuspended
		}
		return existing.ID, nil
	}
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// googlePlaceholderHash produces a valid PBKDF2 digest of a value nobody
// knows. The account therefore exists with a password_hash that satisfies the
// schema, but the password form cannot sign into it until a password is set
// through the recovery flow.
func googlePlaceholderHash() (string, error) {
	secret, err := googleauth.RandomToken()
	if err != nil {
		return "", err
	}
	return security.HashPassword("google-sign-in:" + secret)
}

func googleDisplayName(user googleauth.User, email string) string {
	name := strings.TrimSpace(user.Name)
	if name == "" {
		name = strings.TrimSpace(user.GivenName)
	}
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	if utf8.RuneCountInString(name) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	return name
}

func validGoogleEmail(email string) bool {
	if email == "" || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return false
	}
	local, domain, found := strings.Cut(email, "@")
	return found && local != "" && len(local) <= 64 && strings.Contains(domain, ".")
}

// googleCookieSecure matches the session cookie policy: set SECURE_COOKIES=1
// locally when testing over HTTPS, and production always uses secure cookies.
func googleCookieSecure() bool {
	return runtimeconfig.Production() || os.Getenv("SECURE_COOKIES") == "1"
}
