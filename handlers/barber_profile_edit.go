package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// EditProfileHandler lets a barber change their own details, including
// the shop photo customers see and the picture of themselves.
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
		renderProfileForm(w, s, formFromBarber(barber), barber, "")

	case http.MethodPost:
		// The form carries two files now, so the body is capped before
		// anything parses it.
		limitRequestBodyFor(w, r, 2)

		if !requireCSRF(w, r, s) {
			return
		}

		// stored is the barber as they are on disk, which is what the form
		// has to be redrawn from if this submission is refused.
		stored := barber

		form, problem := readBarberForm(r)
		if problem != "" {
			renderProfileForm(w, s, form, stored, problem)

			return
		}

		shop, addedShop, err := replacePhoto(r, "profile_image", barber.ProfileImage)
		if err != nil {
			renderProfileForm(w, s, form, stored, err.Error())

			return
		}

		personal, addedPersonal, err := replacePhoto(r, "personal_image", barber.PersonalImage)
		if err != nil {
			removeImageIgnoringError(addedShop)
			renderProfileForm(w, s, form, stored, err.Error())

			return
		}

		barber.FullName = form.FullName
		barber.PhoneNumber = form.Phone
		barber.Email = form.Email
		barber.ShopName = form.ShopName
		barber.ShopAddress = form.ShopAddress
		barber.ProfileImage = shop
		barber.PersonalImage = personal

		err = storage.UpdateBarber(barber)

		if errors.Is(err, storage.ErrEmailTaken) {
			removeImageIgnoringError(addedShop)
			removeImageIgnoringError(addedPersonal)
			renderProfileForm(w, s, form, stored,
				"That email belongs to another barber.")

			return
		}

		if err != nil {
			removeImageIgnoringError(addedShop)
			removeImageIgnoringError(addedPersonal)
			serverError(w, "update barber profile", err)

			return
		}

		// The photos that were replaced are only unreferenced once the
		// write above has landed.
		if addedShop != "" {
			removeImageIgnoringError(stored.ProfileImage)
		}

		if addedPersonal != "" {
			removeImageIgnoringError(stored.PersonalImage)
		}

		http.Redirect(w, r, "/barber/profile", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// replacePhoto saves one optionally uploaded photo. It returns the path
// to store and, separately, the file it has just written: an empty file
// input is not an error but a request to keep the current photo, and only
// a file that was really written may be deleted again on failure.
func replacePhoto(
	r *http.Request,
	field string,
	current string,
) (stored string, added string, err error) {
	image, err := barberImages.save(r, field)

	switch {
	case errors.Is(err, errNoUpload):
		return current, "", nil

	case err != nil:
		return current, "", err
	}

	return image, image, nil
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

// renderProfileForm shows the barber's own edit form. The photos travel
// alongside the text values rather than inside them, because a file input
// cannot be filled back in after a rejected submission; what is passed is
// the stored barber, so a refused edit still shows the photos on disk
// rather than ones that have just been deleted.
func renderProfileForm(
	w http.ResponseWriter,
	s sessionData,
	form barberForm,
	stored models.Barber,
	problem string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	values := form.values()
	values["profile_image"] = stored.ProfileImage
	values["personal_image"] = stored.PersonalImage

	renderStatus(w, status, "barber/edit-profile.html", formView{
		CSRF:   s.CSRFToken,
		Error:  problem,
		Values: values,
	})
}
