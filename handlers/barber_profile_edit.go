package handlers

import (
	"barber-booking/storage"
	"html/template"
	"net/http"
	"strings"
)

func EditProfileHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/barber/profile/edit" {
		http.NotFound(w, r)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	switch r.Method {

	case http.MethodGet:

		tmpl, err := template.ParseFiles("templates/barber/edit-profile.html")
		if err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, barber); err != nil {
			http.Error(w, "Template Execution Error", http.StatusInternalServerError)
			return
		}

	case http.MethodPost:

		fullName := strings.TrimSpace(r.FormValue("fullname"))
		phone := strings.TrimSpace(r.FormValue("phone"))
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
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
			"/barber/profile",
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
