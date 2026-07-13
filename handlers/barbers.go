package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"barber-booking/storage"
)

func BarberRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case strings.HasSuffix(path, "/edit"):
		EditBarberHandler(w, r)

	case strings.HasSuffix(path, "/update"):
		UpdateBarberHandler(w, r)

	default:
		BarberProfileHandler(w, r)
	}
}

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

func UpdateBarberHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

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

	fullname := r.FormValue("fullname")
	email := r.FormValue("email")
	phone := r.FormValue("phone")
	shopName := r.FormValue("shop")
	shopAddress := r.FormValue("shopaddress")

	barber.FullName = fullname
	barber.Email = email
	barber.PhoneNumber = phone
	barber.ShopName = shopName
	barber.ShopAddress = shopAddress

	http.Redirect(w, r, fmt.Sprintf("/admin/barbers/%d", barber.ID), http.StatusSeeOther)
}

func ConfirmDeleteBarberHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	parts := strings.Split(path, "/")

	if len(parts) != 5 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)
		return
	}

	barber, found := storage.FindBarberByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFiles("templates/confirm-delete.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, barber)
}
