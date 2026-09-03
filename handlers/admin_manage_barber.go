package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func ListBarbersHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles(
		"templates/admin/barber-list.html",
	)
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
	Barber     *models.Barber
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

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	parts := strings.Split(path, "/")

	if len(parts) != 5 {
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

	tmpl, err := template.ParseFiles(
		"templates/admin/barber-form.html",
	)
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

	if len(parts) != 5 {
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

	fullName := strings.TrimSpace(r.FormValue("fullname"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	shopName := strings.TrimSpace(r.FormValue("shopname"))
	shopAddress := strings.TrimSpace(r.FormValue("shopaddress"))

	if fullName == "" ||
		phone == "" ||
		email == "" ||
		shopName == "" ||
		shopAddress == "" {

		http.Error(
			w,
			"All fields are required",
			http.StatusBadRequest,
		)
		return
	}

	for _, existingBarber := range storage.Barbers {

		if existingBarber.ID != barber.ID &&
			strings.EqualFold(existingBarber.Email, email) {

			http.Error(
				w,
				"Email is already registered",
				http.StatusConflict,
			)
			return
		}
	}

	barber.FullName = fullName
	barber.PhoneNumber = phone
	barber.Email = email
	barber.ShopName = shopName
	barber.ShopAddress = shopAddress

	http.Redirect(
		w,
		r,
		fmt.Sprintf("/admin/barbers/%d", barber.ID),
		http.StatusSeeOther,
	)
}

func ConfirmDeleteBarberHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

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

	tmpl, err := template.ParseFiles(
		"templates/admin/confirm-delete.html",
	)
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

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

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

	barber, found := storage.FindBarberByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	for _, hairstyle := range storage.Hairstyles {

		if hairstyle.BarberID == barber.ID && hairstyle.Image != "" {

			imagePath := strings.TrimPrefix(
				hairstyle.Image,
				"/",
			)

			if err := os.Remove(imagePath); err != nil &&
				!os.IsNotExist(err) {

				http.Error(
					w,
					"Could not delete hairstyle image",
					http.StatusInternalServerError,
				)
				return
			}
		}
	}

	var remainingHairstyles []models.Hairstyle

	for _, hairstyle := range storage.Hairstyles {

		if hairstyle.BarberID != barber.ID {
			remainingHairstyles = append(
				remainingHairstyles,
				hairstyle,
			)
		}
	}

	storage.Hairstyles = remainingHairstyles

	for i, existingBarber := range storage.Barbers {

		if existingBarber.ID == barber.ID {

			storage.Barbers = append(
				storage.Barbers[:i],
				storage.Barbers[i+1:]...,
			)

			http.Redirect(
				w,
				r,
				"/admin/barbers",
				http.StatusSeeOther,
			)

			return
		}
	}

	http.NotFound(w, r)
}