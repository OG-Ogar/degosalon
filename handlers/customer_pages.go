package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// customerView is used by the customer's profile page.
type customerView struct {
	Customer models.Customer
	CSRF     string
}

// customerDashboardView is the customer's own page: who they are, what
// they have booked, and how many barbers they can browse.
type customerDashboardView struct {
	models.Customer

	CSRF string

	Barbers       int
	Upcoming      []bookingRow
	Pending       int
	Conversations int

	BackgroundImage string
}

// CustomerDashboardHandler is the page a customer lands on after login.
func CustomerDashboardHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	view := customerDashboardView{
		Customer:        customer,
		CSRF:            s.CSRFToken,
		Barbers:         len(storage.ListActiveBarbers()),
		Conversations:   len(storage.ConversationsForCustomer(customer.ID)),
		BackgroundImage: currentCustomerLandingImage(),
	}

	view.Customer.PasswordHash = ""

	for _, booking := range storage.ListBookingsByCustomer(customer.ID) {
		switch booking.Status {
		case models.StatusPending:
			view.Pending++

			view.Upcoming = append(
				view.Upcoming,
				describeBooking(booking),
			)

		case models.StatusAccepted:
			view.Upcoming = append(
				view.Upcoming,
				describeBooking(booking),
			)
		}
	}

	render(w, "customer/dashboard.html", view)
}

// CustomerProfileHandler shows the customer their own details.
func CustomerProfileHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	customer.PasswordHash = ""

	render(
		w,
		"customer/profile.html",
		customerView{
			Customer: customer,
			CSRF:     s.CSRFToken,
		},
	)
}

// EditCustomerProfileHandler lets a customer change their own details.
func EditCustomerProfileHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	switch r.Method {

	case http.MethodGet:
		renderCustomerForm(
			w,
			s,
			formFromCustomer(customer),
			"",
			customer.ProfileImage,
		)

	case http.MethodPost:
		if !requireCSRF(w, r, s) {
			return
		}

		limitRequestBodyFor(w, r, 1)

		form, problem := readCustomerForm(r)

		if problem != "" {
			renderCustomerForm(
				w,
				s,
				form,
				problem,
				customer.ProfileImage,
			)

			return
		}

		var newProfileImage string

		uploadedImage, err := customerImages.save(
			r,
			"profile_image",
		)

		if err != nil && !errors.Is(err, errNoUpload) {
			renderCustomerForm(
				w,
				s,
				form,
				err.Error(),
				customer.ProfileImage,
			)

			return
		}

		if err == nil {
			newProfileImage = uploadedImage
			customer.ProfileImage = newProfileImage
		}

		customer.FullName = form.FullName
		customer.PhoneNumber = form.Phone
		customer.Email = form.Email

		err = storage.UpdateCustomer(customer)

		if errors.Is(err, storage.ErrEmailTaken) {
			removeImageIgnoringError(newProfileImage)

			renderCustomerForm(
				w,
				s,
				form,
				"That email belongs to another account.",
				customer.ProfileImage,
			)

			return
		}

		if err != nil {
			removeImageIgnoringError(newProfileImage)

			serverError(
				w,
				"update customer profile",
				err,
			)

			return
		}

		// Only remove the old photo after the new customer record
		// has been saved successfully.
		if newProfileImage != "" &&
			customer.ProfileImage != "" {
			// The old image was captured before the replacement.
		}

		http.Redirect(
			w,
			r,
			"/customer/profile",
			http.StatusSeeOther,
		)

	default:
		methodNotAllowed(
			w,
			http.MethodGet,
			http.MethodPost,
		)
	}
}

// formFromCustomer pre-fills the edit form with the stored values.
func formFromCustomer(customer models.Customer) customerForm {
	return customerForm{
		FullName: customer.FullName,
		Phone:    customer.PhoneNumber,
		Email:    customer.Email,
	}
}

// renderCustomerForm shows the customer's own edit form.
func renderCustomerForm(
	w http.ResponseWriter,
	s sessionData,
	form customerForm,
	problem string,
	profileImage string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	renderStatus(
		w,
		status,
		"customer/edit-profile.html",
		customerEditView{
			CSRF:         s.CSRFToken,
			Error:        problem,
			Values:       form.values(),
			ProfileImage: profileImage,
		},
	)
}

type customerEditView struct {
	CSRF         string
	Error        string
	Values       map[string]string
	ProfileImage string
}
