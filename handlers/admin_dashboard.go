package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// adminDashboardView is the overview shown after the admin logs in.
type adminDashboardView struct {
	CSRF string

	Barbers         int
	ActiveBarbers   int
	Customers       int
	Bookings        int
	PendingBookings int

	LandingImage         string
	CustomerLandingImage string

	Error string
}

// AdminDashboardHandler shows counts and the management links.
func AdminDashboardHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	bookings := storage.ListBookings()

	pending := 0

	for _, booking := range bookings {
		if booking.Status == models.StatusPending {
			pending++
		}
	}

	render(w, "admin/dashboard.html", adminDashboardView{
		CSRF:                 s.CSRFToken,
		Barbers:              len(storage.ListBarbers()),
		ActiveBarbers:        len(storage.ListActiveBarbers()),
		Customers:             len(storage.ListCustomers()),
		Bookings:              len(bookings),
		PendingBookings:       pending,
		LandingImage:          currentLandingImage(),
		CustomerLandingImage: currentCustomerLandingImage(),
	})
}

// AdminLandingImageHandler lets the administrator change the image
// displayed behind the landing page.
func AdminLandingImageHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	limitRequestBody(w, r)

	if !requireCSRF(w, r, s) {
		return
	}

	_, err := saveLandingImage(r)

	if errors.Is(err, errNoUpload) {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)

		return
	}

	if err != nil {
		renderAdminDashboardError(w, s, err.Error())

		return
	}

	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

// AdminCustomerLandingImageHandler lets the administrator change the
// image displayed behind the customer dashboard.
func AdminCustomerLandingImageHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	limitRequestBody(w, r)

	if !requireCSRF(w, r, s) {
		return
	}

	_, err := saveCustomerLandingImage(r)

	if errors.Is(err, errNoUpload) {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)

		return
	}

	if err != nil {
		renderAdminDashboardError(w, s, err.Error())

		return
	}

	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

func renderAdminDashboardError(
	w http.ResponseWriter,
	s sessionData,
	problem string,
) {
	renderStatus(
		w,
		http.StatusBadRequest,
		"admin/dashboard.html",
		adminDashboardView{
			CSRF:                 s.CSRFToken,
			Barbers:              len(storage.ListBarbers()),
			ActiveBarbers:        len(storage.ListActiveBarbers()),
			Customers:             len(storage.ListCustomers()),
			Bookings:              len(storage.ListBookings()),
			PendingBookings:       countPendingBookings(),
			LandingImage:          currentLandingImage(),
			CustomerLandingImage: currentCustomerLandingImage(),
			Error:                 problem,
		},
	)
}

// countPendingBookings returns the number of bookings waiting for
// a barber's decision.
func countPendingBookings() int {
	pending := 0

	for _, booking := range storage.ListBookings() {
		if booking.Status == models.StatusPending {
			pending++
		}
	}

	return pending
}

// AdminBookingsHandler lists every booking in the system.
func AdminBookingsHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	render(w, "admin/bookings.html", listView[bookingRow]{
		Items: describeBookings(storage.ListBookings()),
		CSRF:  s.CSRFToken,
	})
}

// adminCustomerRow is one line of the customer table.
type adminCustomerRow struct {
	models.Customer

	Bookings int
}

// AdminCustomersHandler lists registered customers and how many
// bookings each has made.
func AdminCustomersHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	customers := storage.ListCustomers()
	rows := make([]adminCustomerRow, 0, len(customers))

	for _, customer := range customers {
		rows = append(rows, adminCustomerRow{
			Customer: customer,
			Bookings: len(storage.ListBookingsByCustomer(customer.ID)),
		})
	}

	render(w, "admin/customers.html", listView[adminCustomerRow]{
		Items: rows,
		CSRF:  s.CSRFToken,
	})
}