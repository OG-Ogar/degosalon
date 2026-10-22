package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

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

	extendUploadDeadline(w)
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

	extendUploadDeadline(w)
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
	counts, err := storage.AdminDashboardCounts(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	landingImage, err := currentLandingImage(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	customerLandingImage, err := currentCustomerLandingImage(ctx)
	if err != nil {
		return adminDashboardView{}, err
	}
	return adminDashboardView{CSRF: s.CSRFToken, Barbers: counts.Barbers, ActiveBarbers: counts.ActiveBarbers, Customers: counts.Customers, Bookings: counts.Bookings, PendingBookings: counts.PendingBookings, LandingImage: landingImage, CustomerLandingImage: customerLandingImage, Error: problem}, nil
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

	cursor, ok := readPageCursor(w, r)
	if !ok {
		return
	}
	group, ok := readBookingGroup(w, r)
	if !ok {
		return
	}
	page, err := storage.BookingPage(r.Context(), storage.BookingFilter{Group: group, Now: time.Now(), LegacyTimezone: BookingTimezone()}, cursor)
	if err != nil {
		serverError(w, "load admin booking page", err)
		return
	}
	pagination := bookingPagination(r, cursor, page)
	if len(page.Items) == 0 && (cursor.Before > 0 || cursor.After > 0) {
		http.Redirect(w, r, pagination.FirstLink, 303)
		return
	}

	render(w, "admin/bookings.html", listView[bookingRow]{
		Items:      page.Items,
		Pagination: pagination,
		Query:      group,
		CSRF:       s.CSRFToken,
	})
}

// adminCustomerRow is one line of the customer table.
type adminCustomerRow = storage.AdminCustomerCard

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

	cursor, ok := readPageCursor(w, r)
	if !ok {
		return
	}
	search, ok := readListSearch(w, r)
	if !ok {
		return
	}
	page, err := storage.AdminCustomerPage(r.Context(), search, cursor)
	if err != nil {
		serverError(w, "load admin customer page", err)
		return
	}
	pagination := paginationLinks(r, cursor, 0, 0, false, false, false)
	if len(page.Items) > 0 {
		pagination = paginationLinks(r, cursor, page.Items[0].ID, page.Items[len(page.Items)-1].ID, false, page.HasPrevious, page.HasNext)
	} else if cursor.Before > 0 || cursor.After > 0 {
		http.Redirect(w, r, pagination.FirstLink, http.StatusSeeOther)
		return
	}

	render(w, "admin/customers.html", listView[adminCustomerRow]{
		Items:      page.Items,
		Pagination: pagination,
		Query:      search,
		CSRF:       s.CSRFToken,
	})
}
