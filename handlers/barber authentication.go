package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"net/http"
	"strings"

	"barber-booking/session"
	"barber-booking/storage"
)

func BarberLoginHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case http.MethodGet:

		tmpl, err := template.ParseFiles("templates/barber/login.html")
		if err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "Template Execution Error", http.StatusInternalServerError)
			return
		}

	case http.MethodPost:

		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")

		if email == "" || password == "" {
			http.Error(
				w,
				"Email and Password are required",
				http.StatusBadRequest,
			)
			return
		}

		for _, barber := range storage.Barbers {

			if strings.EqualFold(barber.Email, email) &&
				barber.PassWord == hashPassword(password) {

				s := session.NewSession(barber.ID)

				http.SetCookie(w, &http.Cookie{
					Name:     "barber_session",
					Value:    s.ID,
					Path:     "/",
					HttpOnly: true,
				})

				http.Redirect(
					w,
					r,
					"/barber/dashboard",
					http.StatusSeeOther,
				)

				return
			}
		}

		http.Error(
			w,
			"Invalid email or password",
			http.StatusUnauthorized,
		)

	default:

		http.Error(
			w,
			"Method Not Allowed",
			http.StatusMethodNotAllowed,
		)
	}
}
func hashPassword(password string) string {

	hash := sha256.Sum256([]byte(password))

	return hex.EncodeToString(hash[:])
}
