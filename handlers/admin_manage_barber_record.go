package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"html/template"
	"net/http"
	"strings"
)

func NewBarberHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case http.MethodGet:

		tmpl, err := template.ParseFiles("templates/admin/barber-form.html")
		if err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
			return
		}

		tmpl.Execute(w, nil)

	case http.MethodPost:

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

			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		barber := models.Barber{
			ID:          len(storage.Barbers) + 1,
			FullName:    fullName,
			PhoneNumber: phone,
			Email:       email,
			ShopName:    shopName,
			ShopAddress: shopAddress,
			Status:      "Active",
		}

		storage.Barbers = append(storage.Barbers, barber)

		http.Redirect(w, r, "/admin/barbers", http.StatusSeeOther)

	default:

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

	}
}
