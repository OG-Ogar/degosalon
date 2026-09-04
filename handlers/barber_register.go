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
		form, problem := readBarberForm(r)
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

		_, err = storage.CreateBarber(models.Barber{
			FullName:     form.FullName,
			PhoneNumber:  form.Phone,
			Email:        form.Email,
			PasswordHash: hash,
			ShopName:     form.ShopName,
			ShopAddress:  form.ShopAddress,
			Status:       models.BarberActive,
		})

		if errors.Is(err, storage.ErrEmailTaken) {
			barberRegisterError(w, form, "That email is already registered.")

			return
		}

		if err != nil {
			serverError(w, "create barber", err)

			return
		}

		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// barberRegisterError re-shows the signup form with what was typed.
func barberRegisterError(w http.ResponseWriter, form barberForm, problem string) {
	renderStatus(w, http.StatusBadRequest, "barber/register.html", formView{
		Error:  problem,
		Values: form.values(),
	})
}
