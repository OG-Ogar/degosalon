package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"barber-booking/models"
	"barber-booking/storage"
)

func NewHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case http.MethodGet:

		tmpl, err := template.ParseFiles("templates/barber/hairstyle-form.html")
		if err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "Template Execution Error", http.StatusInternalServerError)
			return
		}

	case http.MethodPost:

		barber, found := CurrentBarber(r)
		if !found {
			http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		category := strings.TrimSpace(r.FormValue("category"))
		description := strings.TrimSpace(r.FormValue("description"))
		priceText := strings.TrimSpace(r.FormValue("price"))
		durationText := strings.TrimSpace(r.FormValue("duration"))
		image := strings.TrimSpace(r.FormValue("image"))

		if name == "" ||
			category == "" ||
			description == "" ||
			priceText == "" ||
			durationText == "" {

			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		price, err := strconv.ParseFloat(priceText, 64)
		if err != nil {
			http.Error(w, "Invalid price", http.StatusBadRequest)
			return
		}

		duration, err := strconv.Atoi(durationText)
		if err != nil {
			http.Error(w, "Invalid duration", http.StatusBadRequest)
			return
		}

		hairstyle := models.Hairstyle{
			ID:          len(storage.Hairstyles) + 1,
			BarberID:    barber.ID,
			Name:        name,
			Category:    category,
			Description: description,
			Price:       price,
			Duration:    duration,
			Image:       image,
			IsAvailable: true,
		}

		storage.Hairstyles = append(storage.Hairstyles, hairstyle)

		http.Redirect(w, r, "/barber/hairstyles", http.StatusSeeOther)

	default:

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func ListHairstylesHandler(w http.ResponseWriter, r *http.Request) {

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	var hairstyles []models.Hairstyle

	for _, hairstyle := range storage.Hairstyles {

		if hairstyle.BarberID == barber.ID {

			hairstyles = append(hairstyles, hairstyle)

		}

	}

	tmpl, err := template.ParseFiles("templates/barber/hairstyle-list.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, hairstyles); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}

}

func ViewHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	hairstyle, found := storage.FindHairstyleByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	if hairstyle.BarberID != barber.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmpl, err := template.ParseFiles("templates/barber/hairstyle-profile.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, hairstyle); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}

}

func EditHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	hairstyle, found := storage.FindHairstyleByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	if hairstyle.BarberID != barber.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmpl, err := template.ParseFiles("templates/barber/edit-hairstyle.html")
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, hairstyle); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}

}

func UpdateHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	hairstyle, found := storage.FindHairstyleByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	if hairstyle.BarberID != barber.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	category := strings.TrimSpace(r.FormValue("category"))
	description := strings.TrimSpace(r.FormValue("description"))
	priceText := strings.TrimSpace(r.FormValue("price"))
	durationText := strings.TrimSpace(r.FormValue("duration"))
	image := strings.TrimSpace(r.FormValue("image"))

	if name == "" ||
		category == "" ||
		description == "" ||
		priceText == "" ||
		durationText == "" {

		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	price, err := strconv.ParseFloat(priceText, 64)
	if err != nil {
		http.Error(w, "Invalid Price", http.StatusBadRequest)
		return
	}

	duration, err := strconv.Atoi(durationText)
	if err != nil {
		http.Error(w, "Invalid Duration", http.StatusBadRequest)
		return
	}

	hairstyle.Name = name
	hairstyle.Category = category
	hairstyle.Description = description
	hairstyle.Price = price
	hairstyle.Duration = duration
	hairstyle.Image = image

	hairstyle.IsAvailable = r.FormValue("available") == "on"

	http.Redirect(
		w,
		r,
		fmt.Sprintf("/barber/hairstyles/%d", hairstyle.ID),
		http.StatusSeeOther,
	)

}
