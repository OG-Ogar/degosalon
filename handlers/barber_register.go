package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/security"
	"barber-booking/storage"
)

// BarberRegisterHandler lets a barber create their own account.
func BarberRegisterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		render(w, "barber/register.html", newFormView(""))

	case http.MethodPost:
		// The registration form accepts one profile photo, so the upload
		// allowance is set before the shared form parse runs. Otherwise the
		// ordinary form limit would reject a photo taken on a phone.
		limitRequestBodyFor(w, r, 1)

		if !beginAuthAttempt(w, r, "register") {
			return
		}

		form, problem := readBarberForm(r)
		if !authAccountAttempt(w, r, "barber", "register", form.Email) {
			return
		}
		password := r.PostFormValue("password")
		confirm := r.PostFormValue("confirm_password")

		switch {
		case problem != "":
			// Keep the message readBarberForm produced.

		case len(password) < minPasswordLength:
			problem = passwordTooShortMessage

		case password != confirm:
			problem = "The two passwords do not match."
		}

		if problem != "" {
			barberRegisterError(w, form, problem)

			return
		}

		hash, err := security.HashPassword(password)
		if err != nil {
			serverError(w, "hash barber password", err)

			return
		}

		// The registration form requires the barber's personal photo.
		personalImage, err := barberImages.save(r, "personal_image")

		if errors.Is(err, errNoUpload) {
			barberRegisterError(
				w,
				form,
				"Please upload your profile photo.",
			)

			return
		}

		if err != nil {
			barberRegisterError(w, form, err.Error())

			return
		}

		_, err = storage.CreateBarberContext(r.Context(), models.Barber{
			FullName:      form.FullName,
			PhoneNumber:   form.Phone,
			Email:         form.Email,
			PasswordHash:  hash,
			ShopName:      form.ShopName,
			ShopAddress:   form.ShopAddress,
			State:         form.State,
			LocalArea:     form.LocalArea,
			PersonalImage: personalImage,
			Status:        models.BarberActive,
		})

		if errors.Is(err, storage.ErrEmailTaken) {
			// The account was not created, so the newly uploaded image is
			// no longer needed.
			removeImageIgnoringError(personalImage)

			barberRegisterError(
				w,
				form,
				"That email is already registered.",
			)

			return
		}

		if err != nil {
			// The account was not created, so clean up the uploaded file.
			removeImageIgnoringError(personalImage)

			serverError(w, "create barber", err)

			return
		}

		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// barberRegisterError re-shows the signup form with what was typed.
func barberRegisterError(
	w http.ResponseWriter,
	form barberForm,
	problem string,
) {
	renderStatus(
		w,
		http.StatusBadRequest,
		"barber/register.html",
		formView{
			Error:  problem,
			Values: form.values(),
		},
	)
}
