package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
)

func preferencesSession(t *testing.T, role session.Role, userID int) session.Session {
	t.Helper()
	recorder := httptest.NewRecorder()
	s, err := session.StartContext(context.Background(), recorder, role, userID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Delete(s.ID) })
	return s
}

func decodePreferences(t *testing.T, recorder *httptest.ResponseRecorder) displayPreferencesBody {
	t.Helper()
	var body displayPreferencesBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode preferences: %v (%s)", err, recorder.Body.String())
	}
	return body
}

func TestDisplayPreferencesHandlerReadsDefaults(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer, err := storage.CreateCustomer(models.Customer{Email: "prefs-read@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	s := preferencesSession(t, session.RoleCustomer, customer.ID)

	recorder := httptest.NewRecorder()
	CustomerPreferencesHandler(recorder, httptest.NewRequest(http.MethodGet, "/customer/settings/preferences", nil), customer, s)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("GET cache control = %q", got)
	}
	body := decodePreferences(t, recorder)
	if body.Theme != models.ThemeForest || body.TextSize != models.TextSizeStandard || body.NotifyAlerts {
		t.Fatalf("default preferences = %+v", body)
	}
}

func TestDisplayPreferencesHandlerWritesAndIsolatesRoles(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer, err := storage.CreateCustomer(models.Customer{Email: "prefs-write@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	barber, err := storage.CreateBarber(models.Barber{Email: "prefs-barber@example.com", ShopName: "Prefs shop"})
	if err != nil {
		t.Fatal(err)
	}
	customerSession := preferencesSession(t, session.RoleCustomer, customer.ID)
	barberSession := preferencesSession(t, session.RoleBarber, barber.ID)

	request := httptest.NewRequest(http.MethodPut, "/customer/settings/preferences", strings.NewReader(`{"theme":"ocean","textSize":"xlarge","notifyAlerts":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", customerSession.CSRFToken)
	recorder := httptest.NewRecorder()
	CustomerPreferencesHandler(recorder, request, customer, customerSession)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if body := decodePreferences(t, recorder); body.Theme != models.ThemeOcean || body.TextSize != models.TextSizeXLarge || !body.NotifyAlerts {
		t.Fatalf("saved preferences = %+v", body)
	}

	// The barber's workspace keeps its own value.
	barberRecorder := httptest.NewRecorder()
	BarberPreferencesHandler(barberRecorder, httptest.NewRequest(http.MethodGet, "/barber/settings/preferences", nil), barber, barberSession)
	if body := decodePreferences(t, barberRecorder); body.Theme != models.ThemeForest || body.TextSize != models.TextSizeStandard {
		t.Fatalf("barber leaked customer preference = %+v", body)
	}
}

func TestDisplayPreferencesHandlerNormalisesCraftedValues(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer, err := storage.CreateCustomer(models.Customer{Email: "prefs-crafted@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	s := preferencesSession(t, session.RoleCustomer, customer.ID)

	request := httptest.NewRequest(http.MethodPut, "/customer/settings/preferences", strings.NewReader(`{"theme":"<svg/onload=1>","textSize":"99"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", s.CSRFToken)
	recorder := httptest.NewRecorder()
	CustomerPreferencesHandler(recorder, request, customer, s)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if body := decodePreferences(t, recorder); body.Theme != models.ThemeForest || body.TextSize != models.TextSizeStandard {
		t.Fatalf("crafted value survived = %+v", body)
	}
}

func TestDisplayPreferencesHandlerRejectsMissingCSRFAndBadInput(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer, err := storage.CreateCustomer(models.Customer{Email: "prefs-csrf@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	s := preferencesSession(t, session.RoleCustomer, customer.ID)

	// No token: refused, and nothing is stored.
	noToken := httptest.NewRequest(http.MethodPut, "/customer/settings/preferences", strings.NewReader(`{"theme":"ocean"}`))
	noToken.Header.Set("Content-Type", "application/json")
	forbidden := httptest.NewRecorder()
	CustomerPreferencesHandler(forbidden, noToken, customer, s)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", forbidden.Code)
	}

	// Malformed JSON.
	badJSON := httptest.NewRequest(http.MethodPut, "/customer/settings/preferences", strings.NewReader(`{"theme":`))
	badJSON.Header.Set("Content-Type", "application/json")
	badJSON.Header.Set("X-CSRF-Token", s.CSRFToken)
	badRecorder := httptest.NewRecorder()
	CustomerPreferencesHandler(badRecorder, badJSON, customer, s)
	if badRecorder.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON status=%d", badRecorder.Code)
	}

	// An unsupported method.
	notAllowed := httptest.NewRecorder()
	CustomerPreferencesHandler(notAllowed, httptest.NewRequest(http.MethodDelete, "/customer/settings/preferences", nil), customer, s)
	if notAllowed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE status=%d", notAllowed.Code)
	}
	if allow := notAllowed.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) || !strings.Contains(allow, http.MethodPut) {
		t.Fatalf("Allow = %q", allow)
	}

	// The refusals above must not have written a preference.
	stored, err := storage.DisplayPreferencesContext(context.Background(), string(session.RoleCustomer), customer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored != models.DefaultDisplayPreferences() {
		t.Fatalf("rejected request stored a value: %+v", stored)
	}
}

func TestPreferencesRouteRequiresAuthentication(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	cases := []struct {
		name     string
		router   http.HandlerFunc
		path     string
		location string
	}{
		{"customer", CustomerRouter, "/customer/settings/preferences", "/customer/login"},
		{"barber", BarberRouter, "/barber/settings/preferences", "/barber/login"},
		{"admin", AdminRouter, "/admin/settings/preferences", "/admin/login"},
	}
	for _, tc := range cases {
		recorder := httptest.NewRecorder()
		tc.router(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != tc.location {
			t.Fatalf("%s: status=%d location=%q", tc.name, recorder.Code, recorder.Header().Get("Location"))
		}
	}
}

func TestPreferencesRouteDispatch(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer, err := storage.CreateCustomer(models.Customer{Email: "prefs-route@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	s, err := session.StartContext(context.Background(), recorder, session.RoleCustomer, customer.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Delete(s.ID) })

	request := httptest.NewRequest(http.MethodGet, "/customer/settings/preferences", nil)
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	CustomerRouter(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("router dispatch status=%d body=%s", response.Code, response.Body.String())
	}
	if body := decodePreferences(t, response); body.Theme != models.ThemeForest {
		t.Fatalf("router dispatch body = %+v", body)
	}
}
