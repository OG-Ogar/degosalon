package handlers

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"barber-booking/reset"
	"barber-booking/session"
	"barber-booking/storage"
)

// submit posts a form and returns the status alongside the body, for the
// cases where being rejected is the point of the test. The post helper
// insists on a 200 and so cannot be used for those.
func submit(
	t *testing.T,
	browser *http.Client,
	target string,
	form url.Values,
) (int, string) {
	t.Helper()

	response, err := browser.PostForm(target, form)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("POST %s: read body: %v", target, err)
	}

	return response.StatusCode, string(body)
}

// registerCustomer creates an account through the real form and returns
// the ID it was stored under.
func registerCustomer(
	t *testing.T,
	browser *http.Client,
	base string,
	email string,
	password string,
) int {
	t.Helper()

	post(t, browser, base+"/customer/register", url.Values{
		"fullname":         {"Ada Lovelace"},
		"phone":            {"0700000000"},
		"email":            {email},
		"password":         {password},
		"confirm_password": {password},
	})

	customer, found := storage.FindCustomerByEmail(email)
	if !found {
		t.Fatalf("registering %s stored no customer", email)
	}

	return customer.ID
}

// TestForgottenPasswordAnswersTheSameForAnyAddress is the check that the
// form cannot be used to work out who has an account here.
func TestForgottenPasswordAnswersTheSameForAnyAddress(t *testing.T) {
	server := newServer(t)
	reset.Clear()

	browser := newBrowser(t)
	registerCustomer(t, browser, server.URL, "ada@example.com", "correct horse")

	known := post(t, browser, server.URL+"/customer/forgot-password",
		url.Values{"email": {"ada@example.com"}})

	unknown := post(t, browser, server.URL+"/customer/forgot-password",
		url.Values{"email": {"nobody@example.com"}})

	mustContain(t, "registered address", known, "a reset link")
	mustContain(t, "unregistered address", unknown, "a reset link")

	// Beyond the address echoed back, the two pages have to be identical.
	if strings.ReplaceAll(known, "ada@example.com", "nobody@example.com") != unknown {
		t.Fatal("the confirmation differs between a registered and an unregistered address")
	}
}

// TestPasswordResetReplacesThePassword walks the whole flow: the new
// password works, the old one stops working, the session that was open
// beforehand is closed, and the link cannot be used twice.
func TestPasswordResetReplacesThePassword(t *testing.T) {
	server := newServer(t)
	reset.Clear()

	browser := newBrowser(t)
	id := registerCustomer(t, browser, server.URL, "ada@example.com", "old password")

	post(t, browser, server.URL+"/customer/forgot-password",
		url.Values{"email": {"ada@example.com"}})

	// The link itself only goes to the server log, so the test takes a
	// token from the package that issues them.
	token, err := reset.New(session.RoleCustomer, id)
	if err != nil {
		t.Fatalf("reset.New: %v", err)
	}

	form := get(t, browser, server.URL+"/customer/reset-password?token="+token)
	mustContain(t, "reset form", form, `name="token"`)

	status, body := submit(t, browser, server.URL+"/customer/reset-password", url.Values{
		"token":            {token},
		"password":         {"new password"},
		"confirm_password": {"something else"},
	})

	if status != http.StatusBadRequest {
		t.Fatalf("mismatched confirmation: status %d, want 400", status)
	}

	mustContain(t, "mismatched confirmation", body, "do not match")

	// A rejected attempt must not have used the link up.
	landing := post(t, browser, server.URL+"/customer/reset-password", url.Values{
		"token":            {token},
		"password":         {"new password"},
		"confirm_password": {"new password"},
	})

	mustContain(t, "after the reset", landing, "Your password has been changed")

	// The session opened at registration belonged to the old password.
	mustContain(t, "session from before the reset",
		get(t, browser, server.URL+"/customer/dashboard"), "Customer login")

	if status, _ := submit(t, browser, server.URL+"/customer/login", url.Values{
		"email":    {"ada@example.com"},
		"password": {"old password"},
	}); status != http.StatusUnauthorized {
		t.Fatalf("old password: status %d, want 401", status)
	}

	mustContain(t, "new password", post(t, browser, server.URL+"/customer/login", url.Values{
		"email":    {"ada@example.com"},
		"password": {"new password"},
	}), "My bookings")

	if status, _ := submit(t, browser, server.URL+"/customer/reset-password", url.Values{
		"token":            {token},
		"password":         {"third password"},
		"confirm_password": {"third password"},
	}); status != http.StatusBadRequest {
		t.Fatalf("reusing the link: status %d, want 400", status)
	}
}

// TestBarberResetPagesRender executes the barber side of the templates,
// which the customer flow above never reaches. A field the handler does
// not pass would show up here as a 500.
func TestBarberResetPagesRender(t *testing.T) {
	server := newServer(t)
	reset.Clear()

	browser := newBrowser(t)

	mustContain(t, "barber request page",
		get(t, browser, server.URL+"/barber/forgot-password"), "Forgotten password")

	status, body := submit(t, browser, server.URL+"/barber/reset-password", url.Values{
		"token":            {"0123456789abcdef"},
		"password":         {"long enough"},
		"confirm_password": {"long enough"},
	})

	if status != http.StatusBadRequest {
		t.Fatalf("unknown token: status %d, want 400", status)
	}

	mustContain(t, "unknown token", body, "expired")
}
