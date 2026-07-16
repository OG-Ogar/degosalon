package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"html/template"
	"net/http"
	"strconv"
	"strings"
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

		barberID, err := strconv.Atoi(r.FormValue("barberid"))
		if err != nil {
			http.Error(w, "Invalid Barber ID", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		description := strings.TrimSpace(r.FormValue("description"))
		priceText := strings.TrimSpace(r.FormValue("price"))
		image := strings.TrimSpace(r.FormValue("image"))

		if name == "" || description == "" || priceText == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		price, err := strconv.ParseFloat(priceText, 64)
		if err != nil {
			http.Error(w, "Invalid Price", http.StatusBadRequest)
			return
		}

		hairstyle := models.Hairstyle{
			ID:          len(storage.Hairstyles) + 1,
			BarberID:    barberID,
			Name:        name,
			Description: description,
			Price:       price,
			Image:       image,
			Status:      "Available",
		}

		storage.Hairstyles = append(storage.Hairstyles, hairstyle)

		http.Redirect(w, r, "/barber/hairstyles", http.StatusSeeOther)

	default:

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}
