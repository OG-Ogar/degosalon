package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// customerEditView is used by the customer's profile edit page.
type customerEditView struct {
	formView

	ProfileImage string
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
	CartCount     int

	ProductOrders []models.ProductOrder

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
		CartCount:       storage.CartItemCount(customer.ID),
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

		oldProfileImage := customer.ProfileImage
		newProfileImage := ""

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
			customer.ProfileImage = uploadedImage
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
				oldProfileImage,
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

		if newProfileImage != "" &&
			oldProfileImage != "" &&
			oldProfileImage != newProfileImage {
			removeImageIgnoringError(oldProfileImage)
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

// formFromCustomer pre-fills the edit form with the customer's
// currently stored details.
func formFromCustomer(customer models.Customer) customerForm {
	return customerForm{
		FullName: customer.FullName,
		Phone:    customer.PhoneNumber,
		Email:    customer.Email,
	}
}

// renderCustomerForm shows the customer's profile edit form.
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
			formView: formView{
				CSRF:   s.CSRFToken,
				Error:  problem,
				Values: form.values(),
			},
			ProfileImage: profileImage,
		},
	)
}