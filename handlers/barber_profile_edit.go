package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// EditProfileHandler lets a barber change their own details, including
// the shop photo customers see.
//
// The write goes through storage.UpdateBarber. Previously it assigned
// through a pointer into the barbers slice, which silently lost the edit
// whenever a concurrent append had moved that slice.
func EditProfileHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	switch r.Method {

	case http.MethodGet:
		renderProfileForm(w, s, formFromBarber(barber), barber.ProfileImage, "")

	case http.MethodPost:
		// The form carries a file now, so the body is capped before
		// anything parses it.
		limitRequestBody(w, r)

		if !requireCSRF(w, r, s) {
			return
		}

		form, problem := readBarberForm(r)
		if problem != "" {
			renderProfileForm(w, s, form, barber.ProfileImage, problem)

			return
		}

		image, err := barberImages.save(r, "profile_image")

		switch {
		case errors.Is(err, errNoUpload):
			image = barber.ProfileImage

		case err != nil:
			renderProfileForm(w, s, form, barber.ProfileImage, err.Error())

			return
		}

		// Whichever way the write goes, exactly one of these two files
		// ends up unreferenced and has to be unlinked.
		previous := barber.ProfileImage
		uploaded := ""

		if image != previous {
			uploaded = image
		}

		barber.FullName = form.FullName
		barber.PhoneNumber = form.Phone
		barber.Email = form.Email
		barber.ShopName = form.ShopName
		barber.ShopAddress = form.ShopAddress
		barber.ProfileImage = image

		err = storage.UpdateBarber(barber)

		if errors.Is(err, storage.ErrEmailTaken) {
			removeImageIgnoringError(uploaded)
			renderProfileForm(w, s, form, previous,
				"That email belongs to another barber.")

			return
		}

		if err != nil {
			removeImageIgnoringError(uploaded)
			serverError(w, "update barber profile", err)

			return
		}

		if uploaded != "" {
			removeImageIgnoringError(previous)
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

// renderProfileForm shows the barber's own edit form. The current photo
// travels alongside the text values rather than inside them, because a
// file input cannot be filled back in after a rejected submission.
func renderProfileForm(
	w http.ResponseWriter,
	s sessionData,
	form barberForm,
	image string,
	problem string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	values := form.values()
	values["profile_image"] = image

	renderStatus(w, status, "barber/edit-profile.html", formView{
		CSRF:   s.CSRFToken,
		Error:  problem,
		Values: values,
	})
}
