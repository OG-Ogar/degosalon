package handlers

import (
	"net/http"

	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// CustomerLoginHandler shows and processes the customer login.
func CustomerLoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		render(w, "customer/login.html", newFormView(""))

	case http.MethodPost:
		email := normalisedEmail(r.PostFormValue("email"))
		password := r.PostFormValue("password")

		if email == "" || password == "" {
			loginError(w, "customer/login.html", email,
				"Email and password are both required.")

			return
		}

		customer, found := storage.FindCustomerByEmail(email)
		if !found {
			// Same cost as a real verification, so response time does not
			// reveal which emails have accounts.
			security.DummyVerify(password)

			loginError(w, "customer/login.html", email,
				"Invalid email or password.")

			return
		}

		if !security.VerifyPassword(customer.PasswordHash, password) {
			loginError(w, "customer/login.html", email,
				"Invalid email or password.")

			return
		}

		if _, err := session.Start(w, session.RoleCustomer, customer.ID); err != nil {
			serverError(w, "start customer session", err)

			return
		}

		http.Redirect(w, r, "/customer/dashboard", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// CustomerLogoutHandler ends the customer session. POST plus a CSRF
// token, so another site cannot sign the visitor out.
func CustomerLogoutHandler(w http.ResponseWriter, r *http.Request, s sessionData) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	session.Clear(w, r, session.RoleCustomer)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
