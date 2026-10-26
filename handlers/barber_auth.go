package handlers

import (
	"errors"
	"net/http"

	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// BarberLoginHandler shows and processes the barber login.
//
// Like the customer form, a browser that has signed in before is asked only
// for the password. The remembered account still has to pass the password (or
// passkey) check before a session is created.
func BarberLoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		if r.URL.Query().Get("switch") == "1" {
			if err := forgetRememberedDevice(w, r, session.RoleBarber); err != nil {
				serverError(w, "forget barber device", err)
				return
			}
			http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
			return
		}

		view := newFormView("")

		email, _, userID, found, err := rememberedAccount(w, r, session.RoleBarber)
		if err != nil {
			serverError(w, "load remembered barber", err)
			return
		}
		if found {
			view.RememberedEmail = email
			view.Values["email"] = email

			count, err := storage.PasskeyCountContext(r.Context(), session.RoleBarber, userID)
			if err != nil {
				serverError(w, "count barber passkeys", err)
				return
			}
			view.PasskeyAvailable = count > 0
		}

		if r.URL.Query().Get("reset") == "1" {
			view.Notice = "Your password has been changed. Sign in with it now."
		}

		render(w, "barber/login.html", view)

	case http.MethodPost:
		if !beginAuthAttempt(w, r, "login") {
			return
		}
		email := normalisedEmail(r.PostFormValue("email"))
		if !authAccountAttempt(w, r, "barber", "login", email) {
			return
		}
		password := r.PostFormValue("password")
		remembered := r.PostFormValue("remembered") == "1" && email != ""
		passkeyAvailable := remembered && r.PostFormValue("passkey_available") == "1"

		failure := func(message string) {
			view := newFormView("")
			view.Error = message
			view.Values["email"] = email
			if remembered {
				view.RememberedEmail = email
				view.PasskeyAvailable = passkeyAvailable
			}
			renderStatus(w, http.StatusUnauthorized, "barber/login.html", view)
		}

		if email == "" || password == "" {
			failure("Email and password are both required.")
			return
		}

		barber, err := storage.FindBarberByEmailResult(r.Context(), email)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "look up barber login", err)
			return
		}
		if errors.Is(err, storage.ErrNotFound) {
			// Burn the same work a real check would, so the response
			// time does not reveal which emails are registered.
			security.DummyVerify(password)
			failure("Invalid email or password.")
			return
		}

		if !security.VerifyPassword(barber.PasswordHash, password) {
			failure("Invalid email or password.")
			return
		}

		if !barber.IsActive() {
			failure("This account is suspended. Please contact the administrator.")
			return
		}

		if err := startVerifiedAccountSession(w, r, session.RoleBarber, barber.ID, barber.PasswordHash); err != nil {
			if errors.Is(err, session.ErrCredentialsChanged) {
				failure("Your account changed while signing in. Try again with your current password.")
				return
			}
			serverError(w, "start barber session", err)

			return
		}

		rememberDeviceBestEffort(w, r, session.RoleBarber, barber.ID)

		http.Redirect(w, r, "/barber/dashboard", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// BarberLogoutHandler ends the barber session.
//
// Logout is a POST with a CSRF token so another site cannot sign the
// visitor out by embedding a link or an image.
func BarberLogoutHandler(w http.ResponseWriter, r *http.Request, s sessionData) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	if err := session.ClearContext(w, r, session.RoleBarber); err != nil {
		serverError(w, "end barber session", err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
