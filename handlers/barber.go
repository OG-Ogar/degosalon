package handlers

import (
	"html/template"
	"net/http"
	"strings"

	"barber-booking/models"
	"barber-booking/storage"
)

func NewBarberHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {

		tmpl := template.Must(template.ParseFiles("templates/new-barber.html"))
		tmpl.Execute(w, nil)
		return
	}

	if r.Method == http.MethodPost {

		fullName := strings.TrimSpace(r.FormValue("fullname"))
		phone := strings.TrimSpace(r.FormValue("phone"))
		email := strings.TrimSpace(r.FormValue("email"))
		shopName := strings.TrimSpace(r.FormValue("shopname"))
		shopAddress := strings.TrimSpace(r.FormValue("shopaddress"))

		if fullName == "" || phone == "" || email == "" || shopName == "" || shopAddress == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		barber := models.Barber{
			ID:          len(storage.Barbers) +1,
			FullName:    fullName,
			PhoneNumber: phone,
			Email:       email,
			ShopName:    shopName,
			ShopAdress:  shopAddress,
			Status:      "Active",
		}

		storage.Barbers = append(storage.Barbers, barber)

		http.Redirect(w, r, "/admin/barbers", http.StatusSeeOther)
		return
	}
	
}

func ListBarbersHandler(w http.ResponseWriter, r *http.Request) {
	barbers := storage.Barbers

	tmpl := template.Must(template.ParseFiles("templates/barbers.html"))
	tmpl.Execute(w, barbers)
}