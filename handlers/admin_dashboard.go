package handlers

import (
	"context"
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
func AdminDashboardHandler(w http.ResponseWriter, r *http.Request, s sessionData) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	view, err := loadAdminDashboard(r.Context(), s, "")
	if err != nil {
		serverError(w, "load admin dashboard", err)
		return
	}
	render(w, "admin/dashboard.html", view)
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
	if errors.Is(err, errSiteImageWrite) {
		serverError(w, "save landing image setting", err)
		return
	}

	if errors.Is(err, errNoUpload) {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)

		return
	}

	if err != nil {
		renderAdminDashboardError(w, r, s, err.Error())

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
	if errors.Is(err, errSiteImageWrite) {
		serverError(w, "save customer landing image setting", err)
		return
	}

	if errors.Is(err, errNoUpload) {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)

		return
	}

	if err != nil {
		renderAdminDashboardError(w, r, s, err.Error())

		return
	}

	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

func renderAdminDashboardError(w http.ResponseWriter, r *http.Request, s sessionData, problem string) {
	view, err := loadAdminDashboard(r.Context(), s, problem)
	if err != nil {
		serverError(w, "load admin dashboard after rejected upload", err)
		return
	}
	renderStatus(w, http.StatusBadRequest, "admin/dashboard.html", view)
}

// countPendingBookings returns the number of bookings waiting for
// a barber's decision.
func loadAdminDashboard(ctx context.Context, s sessionData, problem string) (adminDashboardView, error) {
	bookings, err := storage.ListBookingsResult(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	barbers, err := storage.ListBarbersResult(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	customers, err := storage.ListCustomersResult(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	pending := 0
	active := 0
	for _, booking := range bookings {
		if booking.Status == models.StatusPending {
			pending++
		}
	}
	for _, barber := range barbers {
		if barber.IsActive() {
			active++
		}
	}
	landingImage, err := currentLandingImage(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	customerLandingImage, err := currentCustomerLandingImage(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	return adminDashboardView{CSRF: s.CSRFToken, Barbers: len(barbers), ActiveBarbers: active, Customers: len(customers), Bookings: len(bookings), PendingBookings: pending, LandingImage: landingImage, CustomerLandingImage: customerLandingImage, Error: problem}, nil
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

	bookings, err := storage.ListBookingsResult(r.Context())
	if err != nil {
		serverError(w, "load admin bookings", err)
		return
	}
	rows, err := describeBookings(r.Context(), bookings)
	if err != nil {
		serverError(w, "load admin booking details", err)
		return
	}
	render(w, "admin/bookings.html", listView[bookingRow]{
		Items: rows,
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

	customers, err := storage.ListCustomersResult(r.Context())
	if err != nil {
		serverError(w, "load admin customers", err)
		return
	}

	rows := make([]adminCustomerRow, 0, len(customers))

	for _, customer := range customers {
		bookings, err := storage.ListBookingsByCustomerResult(r.Context(), customer.ID)
		if err != nil {
			serverError(w, "load admin customer booking count", err)
			return
		}

		rows = append(rows, adminCustomerRow{
			Customer: customer,
			Bookings: len(bookings),
		})
	}

	render(w, "admin/customers.html", listView[adminCustomerRow]{
		Items: rows,
		CSRF:  s.CSRFToken,
	})
}
