package handlers

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"barber-booking/models"
	"barber-booking/storage"
)

const maxPushSubscriptionBytes = 8 << 10

type pushSubscriptionBody struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// BarberPushSubscriptionHandler registers or removes one of the barber's
// browsers for Web Push.
func BarberPushSubscriptionHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData) {
	pushSubscriptionHandler(w, r, "barber", barber.ID, s)
}

// CustomerPushSubscriptionHandler registers or removes one of the customer's
// browsers for Web Push.
func CustomerPushSubscriptionHandler(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData) {
	pushSubscriptionHandler(w, r, "customer", customer.ID, s)
}

func pushSubscriptionHandler(w http.ResponseWriter, r *http.Request, role string, userID int, s sessionData) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodPost, http.MethodDelete)

		return
	}

	if !requirePushCSRF(w, r, s) {
		return
	}

	var body pushSubscriptionBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPushSubscriptionBytes)).Decode(&body); err != nil {
		writePushError(w, http.StatusBadRequest, "The notification subscription could not be read.")

		return
	}

	endpoint := strings.TrimSpace(body.Endpoint)
	if !validPushEndpoint(endpoint) {
		writePushError(w, http.StatusBadRequest, "That notification subscription is not valid.")

		return
	}

	if r.Method == http.MethodDelete {
		if err := storage.DeletePushSubscription(r.Context(), role, userID, endpoint); err != nil {
			serverError(w, "remove push subscription", err)

			return
		}
		w.WriteHeader(http.StatusNoContent)

		return
	}

	p256dh := strings.TrimSpace(body.Keys.P256dh)
	auth := strings.TrimSpace(body.Keys.Auth)
	if !validPushKey(p256dh, 65) || !validPushKey(auth, 16) {
		writePushError(w, http.StatusBadRequest, "That notification subscription is not valid.")

		return
	}

	if err := storage.SavePushSubscription(r.Context(), storage.PushSubscription{
		UserRole: role,
		UserID:   userID,
		Endpoint: endpoint,
		P256DH:   p256dh,
		Auth:     auth,
	}); err != nil {
		serverError(w, "save push subscription", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// validPushEndpoint accepts the HTTPS URL a browser's push service issues.
// The host is restricted to the push services browsers actually use: the
// server makes an outbound request to this URL, so an arbitrary address would
// turn registration into a server-side request forgery primitive.
func validPushEndpoint(endpoint string) bool {
	if endpoint == "" || len(endpoint) > 4096 {
		return false
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	if parsed.Scheme == "http" {
		// No public push service uses plain HTTP. Localhost is kept for
		// development and never resolves outside the container.
		return parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1"
	}
	if parsed.Scheme != "https" {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	for _, suffix := range []string{
		"fcm.googleapis.com",
		"push.services.mozilla.com",
		"notify.windows.com",
		"push.apple.com",
	} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// validPushKey checks the base64url-encoded curve key a browser sends. The
// byte length catches a truncated or unrelated value before it reaches the
// encryption step.
func validPushKey(value string, wantBytes int) bool {
	if value == "" || len(value) > 512 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(value)
	}
	return err == nil && len(decoded) == wantBytes
}

func requirePushCSRF(w http.ResponseWriter, r *http.Request, s sessionData) bool {
	submitted := r.Header.Get("X-CSRF-Token")
	if len(submitted) > 0 && subtle.ConstantTimeCompare([]byte(submitted), []byte(s.CSRFToken)) == 1 {
		return true
	}
	writePushError(w, http.StatusForbidden, "This request has expired. Reload the page and try again.")
	return false
}

func writePushError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
