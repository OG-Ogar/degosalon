package handlers

import (
	"html/template"
	"net/http"
)

func MyProfileHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/barber/profile" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	tmpl, err := template.ParseFiles("templates/barber/profile.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, barber); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}
}