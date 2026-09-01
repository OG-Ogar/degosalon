package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"html/template"
	"net/http"
	"strings"
)

func BarberRegisterHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case http.MethodGet:

		tmpl, err := template.ParseFiles("templates/barber/register.html")
		if err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "Template Execution Error", http.StatusInternalServerError)
			return
		}

	case http.MethodPost:

		fullName := strings.TrimSpace(r.FormValue("fullname"))
		phone := strings.TrimSpace(r.FormValue("phone"))
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		shopName := strings.TrimSpace(r.FormValue("shopname"))
		shopAddress := strings.TrimSpace(r.FormValue("shopaddress"))
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if fullName == "" ||
			phone == "" ||
			email == "" ||
			shopName == "" ||
			shopAddress == "" ||
			password == "" ||
			confirmPassword == "" {

			http.Error(
				w,
				"All fields are required",
				http.StatusBadRequest,
			)
			return
		}

		if password != confirmPassword {
			http.Error(
				w,
				"Passwords do not match",
				http.StatusBadRequest,
			)
			return
		}

		for _, barber := range storage.Barbers {

			if strings.EqualFold(barber.Email, email) {
				http.Error(
					w,
					"Email is already registered",
					http.StatusConflict,
				)
				return
			}
		}

		barber := models.Barber{
			ID:          len(storage.Barbers) + 1,
			FullName:    fullName,
			PhoneNumber: phone,
			Email:       email,
			PassWord:    hashPassword(password),
			ShopName:    shopName,
			ShopAddress: shopAddress,
			Status:      "Active",
		}

		storage.Barbers = append(storage.Barbers, barber)

		http.Redirect(
			w,
			r,
			"/barber/login",
			http.StatusSeeOther,
		)

	default:

		http.Error(
			w,
			"Method Not Allowed",
			http.StatusMethodNotAllowed,
		)
	}
}
