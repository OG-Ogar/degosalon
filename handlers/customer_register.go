package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// customerForm is the validated content of the customer signup and
// profile forms.
type customerForm struct {
	Next     string
	FullName string
	Phone    string
	Email    string
}

// readCustomerForm parses the shared customer fields.
func readCustomerForm(r *http.Request) (customerForm, string) {
	form := customerForm{
		Next:     safeCustomerNext(r.PostFormValue("next")),
		FullName: trimmedField(r.PostFormValue("fullname")),
		Phone:    trimmedField(r.PostFormValue("phone")),
		Email:    normalisedEmail(r.PostFormValue("email")),
	}

	if form.FullName == "" || form.Phone == "" || form.Email == "" {
		return form, "Your name, phone number and email are all required."
	}

	return form, ""
}

// values feeds the form back to the template after a rejection.
func (f customerForm) values() map[string]string {
	return map[string]string{
		"next":     f.Next,
		"fullname": f.FullName,
		"phone":    f.Phone,
		"email":    f.Email,
	}
}

// CustomerRegisterHandler creates a customer account.
func CustomerRegisterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		view := newFormView("")
		view.Values["next"] = safeCustomerNext(r.URL.Query().Get("next"))
		render(w, "customer/register.html", view)

	case http.MethodPost:
		limitRequestBodyFor(w, r, 1)

		form, problem := readCustomerForm(r)
		password := r.PostFormValue("password")
		confirm := r.PostFormValue("confirm_password")

		switch {
		case problem != "":
			// Keep the message readCustomerForm produced.

		case len(password) < minPasswordLength:
			problem = passwordTooShortMessage

		case password != confirm:
			problem = "The two passwords do not match."
		}

		if problem != "" {
			customerFormError(
				w,
				"customer/register.html",
				form,
				problem,
			)

			return
		}

		var profileImage string

		uploadedImage, err := customerImages.save(
			r,
			"profile_image",
		)

		if err != nil && !errors.Is(err, errNoUpload) {
			customerFormError(
				w,
				"customer/register.html",
				form,
				err.Error(),
			)

			return
		}

		if err == nil {
			profileImage = uploadedImage
		}

		hash, err := security.HashPassword(password)
		if err != nil {
			removeImageIgnoringError(profileImage)

			serverError(w, "hash customer password", err)

			return
		}

		customer, err := storage.CreateCustomer(models.Customer{
			FullName:     form.FullName,
			PhoneNumber:  form.Phone,
			Email:        form.Email,
			PasswordHash: hash,
			ProfileImage: profileImage,
		})

		if errors.Is(err, storage.ErrEmailTaken) {
			removeImageIgnoringError(profileImage)

			customerFormError(
				w,
				"customer/register.html",
				form,
				"That email is already registered.",
			)

			return
		}

		if err != nil {
			removeImageIgnoringError(profileImage)

			serverError(w, "create customer", err)

			return
		}

		// Signing them straight in saves a pointless second form.
		if err := startVerifiedAccountSession(w, r, session.RoleCustomer, customer.ID, hash); err != nil {
			// The account and its image are committed; keep the image if login fails.
			if errors.Is(err, session.ErrCredentialsChanged) {
				http.Redirect(w, r, customerLoginPath(form.Next), http.StatusSeeOther)
				return
			}

			serverError(
				w,
				"start customer session",
				err,
			)

			return
		}

		destination := form.Next
		if destination == "" {
			destination = "/customer/dashboard"
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// customerFormError re-shows a customer form with what was typed.
func customerFormError(
	w http.ResponseWriter,
	template string,
	form customerForm,
	problem string,
) {
	renderStatus(
		w,
		http.StatusBadRequest,
		template,
		formView{
			Error:  problem,
			Values: form.values(),
		},
	)
}
