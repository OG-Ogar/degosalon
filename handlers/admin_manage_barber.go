package handlers

import (
	"fmt"
    "barber-booking/models"
    "barber-booking/storage"
    "html/template"
    "net/http"
    "strconv"
    "strings"
)
func ListBarbersHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/admin/barber-list.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, storage.Barbers); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}

}

type AdminBarberProfile struct {
	Barber    *models.Barber
	Hairstyles []models.Hairstyle
}

func BarberProfileHandler(w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path

	parts := strings.Split(path, "/")

	if len(parts) != 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)
		return
	}

	barber, found := storage.FindBarberByID(id)
	if !found {
		http.Error(w, "Barber not found", http.StatusNotFound)
		return
	}

	var hairstyles []models.Hairstyle

	for _, hairstyle := range storage.Hairstyles {

		if hairstyle.BarberID == barber.ID {
			hairstyles = append(hairstyles, hairstyle)
		}
	}

	data := AdminBarberProfile{
		Barber:     barber,
		Hairstyles: hairstyles,
	}

	tmpl, err := template.ParseFiles(
		"templates/admin/barber-profile.html",
	)
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution failed", http.StatusInternalServerError)
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

	tmpl, err := template.ParseFiles("templates/admin/barber-form.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, barber); err != nil {
		http.Error(w, "Template execution failed", http.StatusInternalServerError)
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

	fullname := strings.TrimSpace(r.FormValue("fullname"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	shopName := strings.TrimSpace(r.FormValue("shop"))
	shopAddress := strings.TrimSpace(r.FormValue("shopaddress"))

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

	tmpl, err := template.ParseFiles("templates/admin/confirm-delete.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, barber); err != nil {
		http.Error(w, "Template execution failed", http.StatusInternalServerError)
		return
	}
}

func DeleteBarberHandler(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Barber ID", http.StatusBadRequest)
		return
	}

	for i, barber := range storage.Barbers {

		if barber.ID == id {

			storage.Barbers = append(
				storage.Barbers[:i],
				storage.Barbers[i+1:]...,
			)

			http.Redirect(w, r, "/admin/barbers", http.StatusSeeOther)
			return
		}
	}

	http.NotFound(w, r)

}
