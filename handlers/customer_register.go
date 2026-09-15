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
	FullName string
	Phone    string
	Email    string
}

// readCustomerForm parses the shared customer fields.
func readCustomerForm(r *http.Request) (customerForm, string) {
	form := customerForm{
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
		"fullname": f.FullName,
		"phone":    f.Phone,
		"email":    f.Email,
	}
}

// CustomerRegisterHandler creates a customer account.
func CustomerRegisterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		render(w, "customer/register.html", newFormView(""))

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
		if _, err := session.Start(
			w,
			session.RoleCustomer,
			customer.ID,
		); err != nil {
			removeImageIgnoringError(profileImage)

			serverError(
				w,
				"start customer session",
				err,
			)

			return
		}

		http.Redirect(
			w,
			r,
			"/customer/dashboard",
			http.StatusSeeOther,
		)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// CustomerLoginHandler shows and processes the customer login.
func CustomerLoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		view := newFormView("")

		// Set by the last step of a password reset, so the visitor is
		// told the change worked instead of landing on a bare form.
		if r.URL.Query().Get("reset") == "1" {
			view.Notice = "Your password has been changed. Sign in with it now."
		}

		render(w, "customer/login.html", view)

	case http.MethodPost:
		email := normalisedEmail(r.PostFormValue("email"))
		password := r.PostFormValue("password")

		if email == "" || password == "" {
			loginError(
				w,
				"customer/login.html",
				email,
				"Email and password are both required.",
			)

			return
		}

		customer, found := storage.FindCustomerByEmail(email)

		if !found {
			security.DummyVerify(password)

			loginError(
				w,
				"customer/login.html",
				email,
				"Invalid email or password.",
			)

			return
		}

		if !security.VerifyPassword(
			customer.PasswordHash,
			password,
		) {
			loginError(
				w,
				"customer/login.html",
				email,
				"Invalid email or password.",
			)

			return
		}

		if _, err := session.Start(
			w,
			session.RoleCustomer,
			customer.ID,
		); err != nil {
			serverError(
				w,
				"start customer session",
				err,
			)

			return
		}

		http.Redirect(
			w,
			r,
			"/customer/dashboard",
			http.StatusSeeOther,
		)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// CustomerLogoutHandler ends the customer session. POST plus a CSRF
// token, so another site cannot sign the visitor out.
func CustomerLogoutHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	session.Clear(w, r, session.RoleCustomer)

	http.Redirect(
		w,
		r,
		"/",
		http.StatusSeeOther,
	)
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
