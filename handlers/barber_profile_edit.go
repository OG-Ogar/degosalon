package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// EditProfileHandler lets a barber change their own details.
//
// The write now goes through storage.UpdateBarber. Previously it
// assigned through a pointer into the barbers slice, which silently lost
// the edit whenever a concurrent append had moved that slice.
func EditProfileHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	switch r.Method {

	case http.MethodGet:
		renderProfileForm(w, s, formFromBarber(barber), "")

	case http.MethodPost:
		if !requireCSRF(w, r, s) {
			return
		}

		form, problem := readBarberForm(r)
		if problem != "" {
			renderProfileForm(w, s, form, problem)

			return
		}

		barber.FullName = form.FullName
		barber.PhoneNumber = form.Phone
		barber.Email = form.Email
		barber.ShopName = form.ShopName
		barber.ShopAddress = form.ShopAddress

		err := storage.UpdateBarber(barber)

		if errors.Is(err, storage.ErrEmailTaken) {
			renderProfileForm(w, s, form, "That email belongs to another barber.")

			return
		}

		if err != nil {
			serverError(w, "update barber profile", err)

			return
		}

		http.Redirect(w, r, "/barber/profile", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// formFromBarber pre-fills the edit form with the stored values.
func formFromBarber(barber models.Barber) barberForm {
	return barberForm{
		FullName:    barber.FullName,
		Phone:       barber.PhoneNumber,
		Email:       barber.Email,
		ShopName:    barber.ShopName,
		ShopAddress: barber.ShopAddress,
	}
}

// renderProfileForm shows the barber's own edit form.
func renderProfileForm(
	w http.ResponseWriter,
	s sessionData,
	form barberForm,
	problem string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	renderStatus(w, status, "barber/edit-profile.html", formView{
		CSRF:   s.CSRFToken,
		Error:  problem,
		Values: form.values(),
	})
}
