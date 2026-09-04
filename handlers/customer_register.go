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
			customerFormError(w, "customer/register.html", form, problem)

			return
		}

		hash, err := security.HashPassword(password)
		if err != nil {
			serverError(w, "hash customer password", err)

			return
		}

		customer, err := storage.CreateCustomer(models.Customer{
			FullName:     form.FullName,
			PhoneNumber:  form.Phone,
			Email:        form.Email,
			PasswordHash: hash,
		})

		if errors.Is(err, storage.ErrEmailTaken) {
			customerFormError(
				w,
				"customer/register.html",
				form,
				"That email is already registered.",
			)

			return
		}

		if err != nil {
			serverError(w, "create customer", err)

			return
		}

		// Signing them straight in saves a pointless second form.
		if _, err := session.Start(w, session.RoleCustomer, customer.ID); err != nil {
			serverError(w, "start customer session", err)

			return
		}

		http.Redirect(w, r, "/customer/dashboard", http.StatusSeeOther)

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
	renderStatus(w, http.StatusBadRequest, template, formView{
		Error:  problem,
		Values: form.values(),
	})
}
