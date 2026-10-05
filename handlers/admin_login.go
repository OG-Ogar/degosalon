package handlers

import (
	"net/http"

	"barber-booking/session"
)

// AdminLoginHandler shows and processes the administrator login.
//
// A successful login now creates a real session and sets a cookie. The
// previous version only redirected, which meant every /admin/ page was
// reachable without logging in at all.
func AdminLoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		render(w, "admin/login.html", newFormView(""))

	case http.MethodPost:
		username := trimmedField(r.PostFormValue("username"))
		password := r.PostFormValue("password")

		if username == "" || password == "" {
			adminLoginError(w, username, "Username and password are both required.")

			return
		}

		if !validAdminCredentials(username, password) {
			adminLoginError(w, username, "Invalid username or password.")

			return
		}

		if _, err := session.StartContext(r.Context(), w, session.RoleAdmin, 0); err != nil {
			serverError(w, "start admin session", err)

			return
		}

		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// adminLoginError re-renders the login form with the reason it failed.
// The message never says which of the two fields was wrong.
func adminLoginError(w http.ResponseWriter, username, message string) {
	view := newFormView("")
	view.Error = message
	view.Values["username"] = username

	renderStatus(w, http.StatusUnauthorized, "admin/login.html", view)
}

// AdminLogoutHandler ends the administrator session.
func AdminLogoutHandler(w http.ResponseWriter, r *http.Request, s sessionData) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	if err := session.ClearContext(w, r, session.RoleAdmin); err != nil {
		serverError(w, "end admin session", err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
