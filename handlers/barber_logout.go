package handlers

import (
	"barber-booking/session"
	"net/http"
)

func LogoutHandler(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie("barber_session")
	if err == nil {

		session.DeleteSession(cookie.Value)

		http.SetCookie(w, &http.Cookie{
			Name:     "barber_session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})

	}

	http.Redirect(w, r, "/barber/login", http.StatusSeeOther)

}
