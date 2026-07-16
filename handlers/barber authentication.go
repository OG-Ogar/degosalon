package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"barber-booking/storage"
	"barber-booking/session"
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
		password := strings.TrimSpace(r.FormValue("password"))

		if email == "" || password == "" {
			http.Error(w, "Email and Password are required", http.StatusBadRequest)
			return
		}

		for _, barber := range storage.Barbers {

			if barber.Email == email {

				s := session.NewSession(barber.ID)

				http.SetCookie(w, &http.Cookie{
					Name:     "barber_session",
					Value:    s.ID,
					Path:     "/",
					HttpOnly: true,
				})

				http.Redirect(w, r, "/barber/dashboard", http.StatusSeeOther)
				return
			}
		}

		fmt.Fprintln(w, "Invalid email or password")

	default:

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}