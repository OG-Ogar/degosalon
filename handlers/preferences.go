package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
)

// maxPreferencesBytes bounds the preference write body. The payload is two
// short strings and a flag, so anything larger is refused before decoding.
const maxPreferencesBytes = 4 << 10

// displayPreferencesBody is the JSON shape shared by the GET response and the
// PUT request (frontreadme.md §2.3A).
type displayPreferencesBody struct {
	Theme        string `json:"theme"`
	TextSize     string `json:"textSize"`
	NotifyAlerts bool   `json:"notifyAlerts"`
}

func newDisplayPreferencesBody(preferences models.DisplayPreferences) displayPreferencesBody {
	return displayPreferencesBody{
		Theme:        preferences.Theme,
		TextSize:     preferences.TextSize,
		NotifyAlerts: preferences.NotifyAlerts,
	}
}

func writeDisplayPreferences(w http.ResponseWriter, status int, preferences models.DisplayPreferences) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(newDisplayPreferencesBody(preferences))
}

func writeDisplayPreferencesError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// CustomerPreferencesHandler reads and writes the signed-in customer's display
// preferences. The same allowlist/round trip backs every role.
func CustomerPreferencesHandler(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData) {
	displayPreferencesHandler(w, r, session.RoleCustomer, customer.ID, s)
}

// BarberPreferencesHandler reads and writes the signed-in barber's display
// preferences.
func BarberPreferencesHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData) {
	displayPreferencesHandler(w, r, session.RoleBarber, barber.ID, s)
}

// AdminPreferencesHandler reads and writes the administrator's display
// preferences. Administrators share user id 0, so the role keeps the row
// separate from a barber or customer preference.
func AdminPreferencesHandler(w http.ResponseWriter, r *http.Request, s sessionData) {
	displayPreferencesHandler(w, r, session.RoleAdmin, s.UserID, s)
}

func displayPreferencesHandler(w http.ResponseWriter, r *http.Request, role session.Role, userID int, s sessionData) {
	switch r.Method {

	case http.MethodGet:
		preferences, err := storage.DisplayPreferencesContext(r.Context(), string(role), userID)
		if err != nil {
			serverError(w, "load display preferences", err)
			return
		}
		writeDisplayPreferences(w, http.StatusOK, preferences)

	case http.MethodPut, http.MethodPost:
		if !requireRequestCSRF(w, r, s) {
			return
		}
		updateDisplayPreferences(w, r, role, userID)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func updateDisplayPreferences(w http.ResponseWriter, r *http.Request, role session.Role, userID int) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPreferencesBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeDisplayPreferencesError(w, http.StatusRequestEntityTooLarge, "The preference payload is too large.")
		} else {
			writeDisplayPreferencesError(w, http.StatusBadRequest, "The preference payload could not be read.")
		}
		return
	}

	// Pointers so a partial update leaves an unsent field untouched.
	var incoming struct {
		Theme        *string `json:"theme"`
		TextSize     *string `json:"textSize"`
		NotifyAlerts *bool   `json:"notifyAlerts"`
	}
	if err := json.Unmarshal(body, &incoming); err != nil {
		writeDisplayPreferencesError(w, http.StatusBadRequest, "Send a JSON object with theme and textSize.")
		return
	}

	current, err := storage.DisplayPreferencesContext(r.Context(), string(role), userID)
	if err != nil {
		serverError(w, "load display preferences", err)
		return
	}
	if incoming.Theme != nil {
		current.Theme = *incoming.Theme
	}
	if incoming.TextSize != nil {
		current.TextSize = *incoming.TextSize
	}
	if incoming.NotifyAlerts != nil {
		current.NotifyAlerts = *incoming.NotifyAlerts
	}

	saved, err := storage.SaveDisplayPreferencesContext(r.Context(), string(role), userID, current)
	if err != nil {
		serverError(w, "save display preferences", err)
		return
	}
	writeDisplayPreferences(w, http.StatusOK, saved)
}

// requireRequestCSRF accepts the token from the X-CSRF-Token header (the JSON
// write from the preference controller) or, failing that, from a form field,
// so a no-JavaScript form post is protected too.
func requireRequestCSRF(w http.ResponseWriter, r *http.Request, s sessionData) bool {
	submitted := r.Header.Get("X-CSRF-Token")
	if submitted == "" {
		if !parseBoundedForm(w, r) {
			return false
		}
		submitted = r.PostFormValue("csrf_token")
	}
	if len(submitted) > 0 && subtle.ConstantTimeCompare([]byte(submitted), []byte(s.CSRFToken)) == 1 {
		return true
	}
	writeDisplayPreferencesError(w, http.StatusForbidden, "This request has expired. Reload the page and try again.")
	return false
}
