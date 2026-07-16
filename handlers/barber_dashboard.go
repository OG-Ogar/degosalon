package handlers

import (
	"html/template"
	"net/http"
)

func BarberDashboardHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/barber/dashboard.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}
}