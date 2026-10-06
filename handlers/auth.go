package handlers

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
)

// sessionData is the signed-in user for the current request. Handlers
// take it as a parameter so the type system shows which ones require
// authentication, and so they can hand the CSRF token to templates.
type sessionData = session.Session

// requireBarber resolves the signed-in barber or sends the visitor to
// the login page. A barber whose account was deleted or suspended while
// they were signed in is logged straight out.
func requireBarber(w http.ResponseWriter, r *http.Request) (models.Barber, session.Session, bool) {
	s, sessionErr := session.FromRequestContext(r, session.RoleBarber)
	if sessionErr != nil && !errors.Is(sessionErr, session.ErrNotFound) {
		serverError(w, "load barber session", sessionErr)
		return models.Barber{}, session.Session{}, false
	}
	if sessionErr != nil {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)

		return models.Barber{}, session.Session{}, false
	}

	barber, err := storage.FindBarberByIDContext(r.Context(), s.UserID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load signed-in barber", err)
		return models.Barber{}, session.Session{}, false
	}
	if errors.Is(err, storage.ErrNotFound) || !barber.IsActive() {
		if err := session.ClearContext(w, r, session.RoleBarber); err != nil {
			serverError(w, "revoke barber session", err)
			return models.Barber{}, session.Session{}, false
		}
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)

		return models.Barber{}, session.Session{}, false
	}

	return barber, s, true
}

// requireCustomer resolves the signed-in customer or sends the visitor
// to the customer login page.
func requireCustomer(w http.ResponseWriter, r *http.Request) (models.Customer, session.Session, bool) {
	s, sessionErr := session.FromRequestContext(r, session.RoleCustomer)
	if sessionErr != nil && !errors.Is(sessionErr, session.ErrNotFound) {
		serverError(w, "load customer session", sessionErr)
		return models.Customer{}, session.Session{}, false
	}
	if sessionErr != nil {
		destination := "/customer/login"
		if r.Method == http.MethodGet {
			destination = customerLoginPath(r.URL.RequestURI())
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)

		return models.Customer{}, session.Session{}, false
	}

	customer, err := storage.FindCustomerByIDResult(r.Context(), s.UserID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load signed-in customer", err)
		return models.Customer{}, session.Session{}, false
	}
	if errors.Is(err, storage.ErrNotFound) {
		if err := session.ClearContext(w, r, session.RoleCustomer); err != nil {
			serverError(w, "revoke customer session", err)
			return models.Customer{}, session.Session{}, false
		}
		destination := "/customer/login"
		if r.Method == http.MethodGet {
			destination = customerLoginPath(r.URL.RequestURI())
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)

		return models.Customer{}, session.Session{}, false
	}

	return customer, s, true
}

// requireAdmin gates the whole admin area. Previously the admin login
// only redirected on success without creating any session, which left
// every /admin/ URL reachable by anyone.
func requireAdmin(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	s, sessionErr := session.FromRequestContext(r, session.RoleAdmin)
	if sessionErr != nil && !errors.Is(sessionErr, session.ErrNotFound) {
		serverError(w, "load admin session", sessionErr)
		return session.Session{}, false
	}
	if sessionErr != nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)

		return session.Session{}, false
	}

	return s, true
}

// requireCSRF checks that a state-changing request carries the token
// belonging to the caller's session, blocking another site from
// submitting forms on the visitor's behalf.
func requireCSRF(w http.ResponseWriter, r *http.Request, s session.Session) bool {
	if !parseBoundedForm(w, r) {
		return false
	}
	submitted := r.PostFormValue("csrf_token")

	if len(submitted) > 0 &&
		subtle.ConstantTimeCompare([]byte(submitted), []byte(s.CSRFToken)) == 1 {

		return true
	}

	journeyError(
		w,
		"This form has expired. Please reload the page and try again.",
		http.StatusForbidden,
	)

	return false
}

// loginError re-renders a login form with the reason it was rejected.
// The message never distinguishes an unknown account from a wrong
// password, so it cannot be used to enumerate registered emails.
func loginError(w http.ResponseWriter, template, email, message string, next ...string) {
	view := newFormView("")
	view.Error = message
	view.Values["email"] = email
	if len(next) > 0 {
		view.Values["next"] = safeCustomerNext(next[0])
	}

	renderStatus(w, http.StatusUnauthorized, template, view)
}
