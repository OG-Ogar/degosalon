package handlers

import (
	"html/template"
	"net/http"
)

func AdminDashboardHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/admin/dashboard.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}
}