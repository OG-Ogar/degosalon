package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"barber-booking/internal/runtimeconfig"
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	passkeyChallengeLifetime = 5 * time.Minute
	passkeyBodyLimit         = 32 << 10
)

// passkeyUser adapts an account to the WebAuthn library. The user handle is
// stable and namespaced by role, because customer 7 and barber 7 are different
// people.
type passkeyUser struct {
	role        session.Role
	id          int
	email       string
	displayName string
	credentials []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%s:%d", u.role, u.id))
}

func (u passkeyUser) WebAuthnName() string { return u.email }

func (u passkeyUser) WebAuthnDisplayName() string {
	if u.displayName != "" {
		return u.displayName
	}
	return u.email
}

func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// webAuthnForRequest builds the relying-party configuration from the deployed
// origin. Development on localhost may use http, which browsers treat as a
// secure context; production requires the configured HTTPS origin.
func webAuthnForRequest(r *http.Request) (*webauthn.WebAuthn, error) {
	origin := strings.TrimSpace(os.Getenv("SITE_URL"))
	if origin == "" {
		origin = strings.TrimSpace(os.Getenv("RENDER_EXTERNAL_URL"))
	}
	if origin == "" {
		if runtimeconfig.Production() {
			return nil, errors.New("SITE_URL is required for passkey sign-in")
		}
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		origin = scheme + "://" + r.Host
	}

	address, err := url.Parse(origin)
	if err != nil || address.Hostname() == "" || address.User != nil || address.Opaque != "" ||
		(address.Path != "" && address.Path != "/") || address.RawQuery != "" || address.ForceQuery || address.Fragment != "" {
		return nil, errors.New("the configured site origin cannot host passkeys")
	}
	if address.Scheme != "https" && !(address.Scheme == "http" && loopbackHost(address.Hostname())) {
		return nil, errors.New("passkeys require the configured HTTPS site origin")
	}

	return webauthn.New(&webauthn.Config{
		RPID:                  address.Hostname(),
		RPDisplayName:         "De Go Salon",
		RPOrigins:             []string{address.Scheme + "://" + address.Host},
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.Platform,
			ResidentKey:             protocol.ResidentKeyRequirementPreferred,
			UserVerification:        protocol.VerificationRequired,
		},
	})
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func passkeyCookieName(role session.Role, purpose string) string {
	return string(role) + "_passkey_" + purpose
}

func setPasskeyChallenge(w http.ResponseWriter, role session.Role, purpose string, data webauthn.SessionData) error {
	if data.Expires.IsZero() {
		data.Expires = time.Now().Add(passkeyChallengeLifetime)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     passkeyCookieName(role, purpose),
		Value:    base64.RawURLEncoding.EncodeToString(payload),
		Path:     "/",
		MaxAge:   int(passkeyChallengeLifetime.Seconds()),
		HttpOnly: true,
		Secure:   authCookieSecure(),
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func readPasskeyChallenge(w http.ResponseWriter, r *http.Request, role session.Role, purpose string) (webauthn.SessionData, bool) {
	cookie, err := r.Cookie(passkeyCookieName(role, purpose))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Start the fingerprint request again.")
		return webauthn.SessionData{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		clearPasskeyChallenge(w, role, purpose)
		jsonError(w, http.StatusBadRequest, "Start the fingerprint request again.")
		return webauthn.SessionData{}, false
	}

	var data webauthn.SessionData
	if err := json.Unmarshal(payload, &data); err != nil || !data.Expires.IsZero() && data.Expires.Before(time.Now()) {
		clearPasskeyChallenge(w, role, purpose)
		jsonError(w, http.StatusBadRequest, "The fingerprint request expired. Try again.")
		return webauthn.SessionData{}, false
	}

	// Single use: any finish clears the challenge, whether it succeeds or not.
	clearPasskeyChallenge(w, role, purpose)
	return data, true
}

func clearPasskeyChallenge(w http.ResponseWriter, role session.Role, purpose string) {
	http.SetCookie(w, &http.Cookie{
		Name:     passkeyCookieName(role, purpose),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   authCookieSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}

func requireCSRFHeader(w http.ResponseWriter, r *http.Request, s sessionData) bool {
	submitted := r.Header.Get("X-CSRF-Token")
	if len(submitted) > 0 && subtle.ConstantTimeCompare([]byte(submitted), []byte(s.CSRFToken)) == 1 {
		return true
	}
	jsonError(w, http.StatusForbidden, "This form has expired. Reload the page and try again.")
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("passkey_json_failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
	}
}

func jsonError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// passkeyAccount carries the password hash so passkey sign-in goes through the
// same credential-checked session path as the password form.
type passkeyAccount struct {
	passkeyUser
	passwordHash string
}

// loadPasskeyAccount loads the displayed account details and its registered
// credentials. A malformed credential row is skipped rather than trusted.
func loadPasskeyAccount(ctx context.Context, role session.Role, userID int) (passkeyAccount, error) {
	account := passkeyAccount{passkeyUser: passkeyUser{role: role, id: userID}}

	switch role {
	case session.RoleCustomer:
		customer, err := storage.FindCustomerByIDResult(ctx, userID)
		if err != nil {
			return passkeyAccount{}, err
		}
		account.email = customer.Email
		account.displayName = customer.FullName
		account.passwordHash = customer.PasswordHash
	case session.RoleBarber:
		barber, err := storage.FindBarberByIDContext(ctx, userID)
		if err != nil {
			return passkeyAccount{}, err
		}
		account.email = barber.Email
		account.displayName = barber.FullName
		account.passwordHash = barber.PasswordHash
	default:
		return passkeyAccount{}, errors.New("unsupported passkey role")
	}

	stored, err := storage.PasskeysForUserContext(ctx, role, userID)
	if err != nil {
		return passkeyAccount{}, err
	}
	for _, passkey := range stored {
		var credential webauthn.Credential
		if err := json.Unmarshal(passkey.Credential, &credential); err != nil {
			slog.Warn("passkey_record_invalid", "role", role, "user_id", userID, "error", err)
			continue
		}
		account.credentials = append(account.credentials, credential)
	}
	return account, nil
}

func passkeyHandleUserID(role session.Role, handle []byte) (int, bool) {
	prefix := string(role) + ":"
	text := string(handle)
	if !strings.HasPrefix(text, prefix) {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(text, prefix))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func passkeyServerError(w http.ResponseWriter, operation string, err error) {
	slog.Error("passkey_request_failed", "request_id", w.Header().Get("X-Request-ID"), "operation", operation, "error", err)
	jsonError(w, http.StatusInternalServerError, "Something went wrong. Please try again.")
}

// CustomerPasskeyRegisterBegin starts fingerprint enrolment for a signed-in
// customer.
func CustomerPasskeyRegisterBegin(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData) {
	passkeyRegisterBegin(w, r, session.RoleCustomer, customer.ID, s)
}

// CustomerPasskeyRegisterFinish stores the credential created on the device.
func CustomerPasskeyRegisterFinish(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData) {
	passkeyRegisterFinish(w, r, session.RoleCustomer, customer.ID, s)
}

// BarberPasskeyRegisterBegin starts fingerprint enrolment for a signed-in
// barber.
func BarberPasskeyRegisterBegin(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData) {
	passkeyRegisterBegin(w, r, session.RoleBarber, barber.ID, s)
}

// BarberPasskeyRegisterFinish stores the credential created on the device.
func BarberPasskeyRegisterFinish(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData) {
	passkeyRegisterFinish(w, r, session.RoleBarber, barber.ID, s)
}

func passkeyRegisterBegin(w http.ResponseWriter, r *http.Request, role session.Role, userID int, s sessionData) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !requireCSRFHeader(w, r, s) {
		return
	}

	lib, err := webAuthnForRequest(r)
	if err != nil {
		slog.Warn("passkey_unavailable", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		jsonError(w, http.StatusServiceUnavailable, "Fingerprint sign-in is not available on this site yet.")
		return
	}

	account, err := loadPasskeyAccount(r.Context(), role, userID)
	if err != nil {
		passkeyServerError(w, "load passkey account", err)
		return
	}

	exclusions := make([]protocol.CredentialDescriptor, 0, len(account.credentials))
	for _, credential := range account.credentials {
		exclusions = append(exclusions, credential.Descriptor())
	}

	creation, challenge, err := lib.BeginRegistration(
		account.passkeyUser,
		webauthn.WithExclusions(exclusions),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.Platform,
			ResidentKey:             protocol.ResidentKeyRequirementPreferred,
			UserVerification:        protocol.VerificationRequired,
		}),
	)
	if err != nil {
		passkeyServerError(w, "begin passkey registration", err)
		return
	}
	if err := setPasskeyChallenge(w, role, "register", *challenge); err != nil {
		passkeyServerError(w, "save passkey registration challenge", err)
		return
	}

	writeJSON(w, http.StatusOK, creation)
}

func passkeyRegisterFinish(w http.ResponseWriter, r *http.Request, role session.Role, userID int, s sessionData) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !requireCSRFHeader(w, r, s) {
		return
	}

	challenge, ok := readPasskeyChallenge(w, r, role, "register")
	if !ok {
		return
	}

	lib, err := webAuthnForRequest(r)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "Fingerprint sign-in is not available on this site yet.")
		return
	}

	account, err := loadPasskeyAccount(r.Context(), role, userID)
	if err != nil {
		passkeyServerError(w, "load passkey account", err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, passkeyBodyLimit)
	credential, err := lib.FinishRegistration(account.passkeyUser, challenge, r)
	if err != nil {
		slog.Warn("passkey_registration_rejected", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		jsonError(w, http.StatusBadRequest, "The fingerprint could not be registered. Please try again.")
		return
	}

	// The attestation certificate is not needed after verification and can
	// be large; keep only the public key, counter and transports.
	credential.Attestation = webauthn.CredentialAttestation{}
	payload, err := json.Marshal(credential)
	if err != nil {
		passkeyServerError(w, "encode passkey", err)
		return
	}

	err = storage.CreatePasskeyContext(r.Context(), models.Passkey{
		CredentialID: credential.ID,
		Role:         string(role),
		UserID:       userID,
		Email:        account.email,
		DisplayName:  account.displayName,
		Credential:   payload,
		CreatedAt:    time.Now(),
	})
	if errors.Is(err, storage.ErrPasskeyExists) {
		jsonError(w, http.StatusConflict, "This fingerprint is already registered.")
		return
	}
	if err != nil {
		passkeyServerError(w, "save passkey", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// CustomerPasskeyLoginBegin starts a fingerprint sign-in from the customer
// login page.
func CustomerPasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	passkeyLoginBegin(w, r, session.RoleCustomer)
}

// CustomerPasskeyLoginFinish verifies the fingerprint and opens the dashboard.
func CustomerPasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	passkeyLoginFinish(w, r, session.RoleCustomer)
}

// BarberPasskeyLoginBegin starts a fingerprint sign-in from the barber login
// page.
func BarberPasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	passkeyLoginBegin(w, r, session.RoleBarber)
}

// BarberPasskeyLoginFinish verifies the fingerprint and opens the dashboard.
func BarberPasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	passkeyLoginFinish(w, r, session.RoleBarber)
}

func passkeyLoginBegin(w http.ResponseWriter, r *http.Request, role session.Role) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !authBudget(w, r, "auth:passkey:peer:"+string(role)+":"+authClientAddress(r), 60, 15*time.Minute) {
		return
	}

	_, _, userID, found, err := rememberedAccount(w, r, role)
	if err != nil {
		passkeyServerError(w, "load remembered account for passkey", err)
		return
	}
	if !found {
		jsonError(w, http.StatusBadRequest, "Sign in with your password once on this device before using a fingerprint.")
		return
	}

	lib, err := webAuthnForRequest(r)
	if err != nil {
		slog.Warn("passkey_unavailable", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		jsonError(w, http.StatusServiceUnavailable, "Fingerprint sign-in is not available on this site yet.")
		return
	}

	account, err := loadPasskeyAccount(r.Context(), role, userID)
	if err != nil {
		passkeyServerError(w, "load passkey account", err)
		return
	}
	if len(account.credentials) == 0 {
		jsonError(w, http.StatusBadRequest, "No fingerprint is set up for this account yet. Sign in with your password, then add one from your dashboard.")
		return
	}

	assertion, challenge, err := lib.BeginLogin(account.passkeyUser)
	if err != nil {
		passkeyServerError(w, "begin passkey login", err)
		return
	}
	if err := setPasskeyChallenge(w, role, "login", *challenge); err != nil {
		passkeyServerError(w, "save passkey login challenge", err)
		return
	}

	writeJSON(w, http.StatusOK, assertion)
}

func passkeyLoginFinish(w http.ResponseWriter, r *http.Request, role session.Role) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !authBudget(w, r, "auth:passkey:peer:"+string(role)+":"+authClientAddress(r), 60, 15*time.Minute) {
		return
	}

	challenge, ok := readPasskeyChallenge(w, r, role, "login")
	if !ok {
		return
	}

	userID, ok := passkeyHandleUserID(role, challenge.UserID)
	if !ok {
		jsonError(w, http.StatusBadRequest, "Start the fingerprint sign-in again.")
		return
	}

	lib, err := webAuthnForRequest(r)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "Fingerprint sign-in is not available on this site yet.")
		return
	}

	account, err := loadPasskeyAccount(r.Context(), role, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			jsonError(w, http.StatusUnauthorized, "That account is no longer available. Sign in with your password.")
			return
		}
		passkeyServerError(w, "load passkey account", err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, passkeyBodyLimit)
	credential, err := lib.FinishLogin(account.passkeyUser, challenge, r)
	if err != nil {
		slog.Warn("passkey_login_rejected", "request_id", w.Header().Get("X-Request-ID"), "role", role, "error", err)
		jsonError(w, http.StatusUnauthorized, "The fingerprint could not be verified. Try again or use your password.")
		return
	}

	if role == session.RoleBarber {
		barber, err := storage.FindBarberByIDContext(r.Context(), userID)
		if err != nil {
			passkeyServerError(w, "load signed-in barber", err)
			return
		}
		if !barber.IsActive() {
			jsonError(w, http.StatusForbidden, "This account is suspended. Please contact the administrator.")
			return
		}
	}

	if err := startVerifiedAccountSession(w, r, role, userID, account.passwordHash); err != nil {
		if errors.Is(err, session.ErrCredentialsChanged) {
			jsonError(w, http.StatusUnauthorized, "Your account changed while signing in. Try again with your current password.")
			return
		}
		passkeyServerError(w, "start passkey session", err)
		return
	}

	// Keep the counter current so a cloned authenticator can be detected
	// on the next sign-in.
	if payload, err := json.Marshal(credential); err == nil {
		if err := storage.UpdatePasskeyUseContext(r.Context(), models.Passkey{
			CredentialID: credential.ID,
			Credential:   payload,
		}, time.Now()); err != nil {
			slog.Warn("passkey_counter_update_failed", "request_id", w.Header().Get("X-Request-ID"), "role", role, "error", err)
		}
	}

	rememberDeviceBestEffort(w, r, role, userID)

	destination := "/customer/dashboard"
	if role == session.RoleBarber {
		destination = "/barber/dashboard"
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "redirect": destination})
}
