package handlers

import (
	"errors"
	"net/http"

	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// CustomerLoginHandler shows and processes the customer login.
func CustomerLoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		view := newFormView("")
		view.Values["next"] = safeCustomerNext(r.URL.Query().Get("next"))

		// Set by the last step of a password reset, so the visitor is
		// told the change worked instead of landing on a bare form.
		if r.URL.Query().Get("reset") == "1" {
			view.Notice = "Your password has been changed. Sign in with it now."
		}

		render(w, "customer/login.html", view)

	case http.MethodPost:
		email := normalisedEmail(r.PostFormValue("email"))
		password := r.PostFormValue("password")
		next := safeCustomerNext(r.PostFormValue("next"))

		if email == "" || password == "" {
			loginError(
				w,
				"customer/login.html",
				email,
				"Email and password are both required.",
				next,
			)

			return
		}

		customer, err := storage.FindCustomerByEmailResult(r.Context(), email)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "look up customer login", err)
			return
		}

		if errors.Is(err, storage.ErrNotFound) {
			// Same cost as a real verification, so response time does not
			// reveal which emails have accounts.
			security.DummyVerify(password)

			loginError(
				w,
				"customer/login.html",
				email,
				"Invalid email or password.",
				next,
			)

			return
		}

		if !security.VerifyPassword(
			customer.PasswordHash,
			password,
		) {
			loginError(
				w,
				"customer/login.html",
				email,
				"Invalid email or password.",
				next,
			)

			return
		}

		if err := startVerifiedAccountSession(w, r, session.RoleCustomer, customer.ID, customer.PasswordHash); err != nil {
			if errors.Is(err, session.ErrCredentialsChanged) {
				loginError(w, "customer/login.html", email, "Your account changed while signing in. Try again with your current password.", next)
				return
			}
			serverError(
				w,
				"start customer session",
				err,
			)

			return
		}

		if next == "" {
			next = "/customer/dashboard"
		}
		http.Redirect(w, r, next, http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// CustomerLogoutHandler ends the customer session. POST plus a CSRF
// token, so another site cannot sign the visitor out.
func CustomerLogoutHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	if err := session.ClearContext(w, r, session.RoleCustomer); err != nil {
		serverError(w, "end customer session", err)
		return
	}

	http.Redirect(
		w,
		r,
		"/",
		http.StatusSeeOther,
	)
}
