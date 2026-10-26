package handlers

import (
	"errors"
	"net/http"

	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// CustomerLoginHandler shows and processes the customer login.
//
// A browser that has signed in before gets a remembered-account form: the
// email is shown as a fixed label and only the password is asked for. The
// remembered cookie identifies an account, not a session, so the password (or
// a registered passkey) is still required.
func CustomerLoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		next := safeCustomerNext(r.URL.Query().Get("next"))

		// "Use another account" forgets the hint and returns to the full
		// email-and-password form.
		if r.URL.Query().Get("switch") == "1" {
			if err := forgetRememberedDevice(w, r, session.RoleCustomer); err != nil {
				serverError(w, "forget customer device", err)
				return
			}
			http.Redirect(w, r, customerLoginPath(next), http.StatusSeeOther)
			return
		}

		view := newFormView("")
		view.Values["next"] = next

		email, _, userID, found, err := rememberedAccount(w, r, session.RoleCustomer)
		if err != nil {
			serverError(w, "load remembered customer", err)
			return
		}
		if found {
			view.RememberedEmail = email
			view.Values["email"] = email

			count, err := storage.PasskeyCountContext(r.Context(), session.RoleCustomer, userID)
			if err != nil {
				serverError(w, "count customer passkeys", err)
				return
			}
			view.PasskeyAvailable = count > 0
		}

		// Set by the last step of a password reset, so the visitor is
		// told the change worked instead of landing on a bare form.
		if r.URL.Query().Get("reset") == "1" {
			view.Notice = "Your password has been changed. Sign in with it now."
		}

		render(w, "customer/login.html", view)

	case http.MethodPost:
		if !beginAuthAttempt(w, r, "login") {
			return
		}
		email := normalisedEmail(r.PostFormValue("email"))
		if !authAccountAttempt(w, r, "customer", "login", email) {
			return
		}
		password := r.PostFormValue("password")
		next := safeCustomerNext(r.PostFormValue("next"))
		remembered := r.PostFormValue("remembered") == "1" && email != ""
		passkeyAvailable := remembered && r.PostFormValue("passkey_available") == "1"

		// Every rejection re-renders the same form. A remembered account
		// keeps its email label and password field instead of falling back
		// to asking for the address again.
		failure := func(message string) {
			view := newFormView("")
			view.Error = message
			view.Values["email"] = email
			view.Values["next"] = next
			if remembered {
				view.RememberedEmail = email
				view.PasskeyAvailable = passkeyAvailable
			}
			renderStatus(w, http.StatusUnauthorized, "customer/login.html", view)
		}

		if email == "" || password == "" {
			failure("Email and password are both required.")
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
			failure("Invalid email or password.")
			return
		}

		if !security.VerifyPassword(
			customer.PasswordHash,
			password,
		) {
			failure("Invalid email or password.")
			return
		}

		if err := startVerifiedAccountSession(w, r, session.RoleCustomer, customer.ID, customer.PasswordHash); err != nil {
			if errors.Is(err, session.ErrCredentialsChanged) {
				failure("Your account changed while signing in. Try again with your current password.")
				return
			}
			serverError(
				w,
				"start customer session",
				err,
			)

			return
		}

		rememberDeviceBestEffort(w, r, session.RoleCustomer, customer.ID)

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
