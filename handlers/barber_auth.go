package handlers

import (
	"errors"
	"net/http"

	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// BarberLoginHandler shows and processes the barber login.
func BarberLoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		view := newFormView("")

		if r.URL.Query().Get("reset") == "1" {
			view.Notice = "Your password has been changed. Sign in with it now."
		}

		render(w, "barber/login.html", view)

	case http.MethodPost:
		email := normalisedEmail(r.PostFormValue("email"))
		password := r.PostFormValue("password")

		if email == "" || password == "" {
			loginError(w, "barber/login.html", email,
				"Email and password are both required.")

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

			loginError(w, "barber/login.html", email,
				"Invalid email or password.")

			return
		}

		if !security.VerifyPassword(barber.PasswordHash, password) {
			loginError(w, "barber/login.html", email,
				"Invalid email or password.")

			return
		}

		if !barber.IsActive() {
			loginError(w, "barber/login.html", email,
				"This account is suspended. Please contact the administrator.")

			return
		}

		if _, err := session.Start(w, session.RoleBarber, barber.ID); err != nil {
			serverError(w, "start barber session", err)

			return
		}

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

	session.Clear(w, r, session.RoleBarber)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
