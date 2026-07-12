package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"barber-booking/storage"
)

func BarberProfileHandler(w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path

	parts := strings.Split(path, "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	idText := parts[3]

	id, err := strconv.Atoi(idText)
	if err != nil {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)
		return
	}

	barber, found := storage.FindBarberByID(id)
	if !found {
		http.Error(w, "Barber not found", http.StatusNotFound)
		return
	}

	tmpl, err := template.ParseFiles("templates/barber-profile.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, barber)
	if err != nil {
		http.Error(w, "Temlate execution failed", http.StatusInternalServerError)
		return
	}
}

func EditBarberHandler(w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path

	parts := strings.Split(path, "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	idText := parts[3]

	id, err := strconv.Atoi(idText)
	if err != nil {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)
		return
	}

	barber, found := storage.FindBarberByID(id)
	if !found {
		http.Error(w, "Barber not found", http.StatusNotFound)
		return
	}

	tmpl, err := template.ParseFiles("templates/edit-barber.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, barber)
	if err != nil {
		http.Error(w, "Temlate execution failed", http.StatusInternalServerError)
		return
	}
}
