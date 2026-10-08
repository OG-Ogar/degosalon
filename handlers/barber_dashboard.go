package handlers

import (
	"net/http"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberDashboardView is the barber's landing page: their name plus the
// counts that tell them whether anything needs attention.
type barberDashboardView struct {
	models.Barber

	CSRF            string
	OrderPagination listPagination
	OrderStatus     string

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

	counts, err := storage.DashboardTotals(r.Context(), storage.BookingFilter{BarberID: barber.ID, Now: time.Now(), LegacyTimezone: BookingTimezone()})
	if err != nil {
		serverError(w, "load barber dashboard totals", err)
		return
	}

	// The password hash must never reach the template.
	barber.PasswordHash = ""

	cursor, ok := readNamedPageCursor(w, r, "orders")
	if !ok {
		return
	}
	status, ok := readFulfillmentFilter(w, r)
	if !ok {
		return
	}
	orderItems, err := storage.SellerOrderItemPage(r.Context(), barber.ID, status, cursor)
	if err != nil {
		serverError(w, "load seller order page", err)
		return
	}
	orderLinks := orderItemPagination(r, cursor, orderItems.Items, orderItems.HasPrevious, orderItems.HasNext)
	if len(orderItems.Items) == 0 && (cursor.After > 0 || cursor.Before > 0) {
		http.Redirect(w, r, orderLinks.FirstLink, 303)
		return
	}

	conversations, err := storage.ConversationCountForBarber(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load dashboard conversations", err)
		return
	}
	render(w, "barber/dashboard.html", barberDashboardView{
		Barber:           barber,
		CSRF:             s.CSRFToken,
		Hairstyles:       counts.Hairstyles,
		PendingBookings:  counts.Pending,
		UpcomingBookings: counts.AcceptedUpcoming,
		Conversations:    conversations,
		ProductOrders:    orderItems.Items,
		OrderPagination:  orderLinks,
		OrderStatus:      status,
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
