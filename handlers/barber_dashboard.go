package handlers

import (
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberDashboardView is the barber's landing page: their name plus the
// counts that tell them whether anything needs attention.
type barberDashboardView struct {
	models.Barber

	CSRF string

	Hairstyles       int
	PendingBookings  int
	UpcomingBookings int
	Conversations    int

	ProductOrders []models.ProductOrderItem
}

// BarberDashboardHandler shows the barber's own overview.
func BarberDashboardHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	bookings, err := storage.ListBookingsByBarberResult(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load dashboard schedule", err)
		return
	}
	styles, err := storage.ListHairstylesByBarberResult(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load dashboard services", err)
		return
	}

	pending := 0
	upcoming := 0

	for _, booking := range bookings {
		switch booking.Status {
		case models.StatusPending:
			pending++
		case models.StatusAccepted:
			upcoming++
		}
	}

	// The password hash must never reach the template.
	barber.PasswordHash = ""

	orderItems, err := storage.ListProductOrderItemsByBarber(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load seller sales", err)
		return
	}

	render(w, "barber/dashboard.html", barberDashboardView{
		Barber:           barber,
		CSRF:             s.CSRFToken,
		Hairstyles:       len(styles),
		PendingBookings:  pending,
		UpcomingBookings: upcoming,
		Conversations:    len(storage.ConversationsForBarber(barber.ID)),
		ProductOrders:    orderItems,
	})
}

// MyProfileHandler shows the barber their own details.
func MyProfileHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	barber.PasswordHash = ""

	render(w, "barber/profile.html", barberView{
		Barber: barber,
		CSRF:   s.CSRFToken,
	})
}
