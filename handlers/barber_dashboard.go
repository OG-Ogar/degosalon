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
	Feed socialFeedView

	CSRF            string
	PasskeyRole     string
	OrderPagination listPagination
	OrderStatus     string

	Hairstyles       int
	PendingBookings  int
	UpcomingBookings int
	Conversations    int

	ProductOrders []models.ProductOrderItem
}

// BarberDashboardHandler shows the barber's social home.
func BarberDashboardHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	actorPhoto := barber.PersonalImage
	if actorPhoto == "" {
		actorPhoto = barber.ProfileImage
	}
	if r.Method == http.MethodPost {
		createSocialPost(w, r, models.SenderBarber, barber.ID, barber.FullName, actorPhoto, s)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	feed, err := loadSocialFeed(r, models.SenderBarber, barber.ID, barber.FullName, actorPhoto, s.CSRFToken)
	if err != nil {
		serverError(w, "load social home feed", err)
		return
	}

	// The password hash must never reach the template.
	barber.PasswordHash = ""

	render(w, "barber/dashboard.html", barberDashboardView{
		Barber: barber,
		CSRF:   s.CSRFToken,
		Feed:   feed,
	})
}

// BarberAccountHandler is the account summary page that the home menu opens.
func BarberAccountHandler(
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
		serverError(w, "load barber account totals", err)
		return
	}
	conversations, err := storage.ConversationCountForBarber(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load account conversations", err)
		return
	}

	// The password hash must never reach the template.
	barber.PasswordHash = ""

	render(w, "barber/account.html", barberDashboardView{
		Barber:           barber,
		CSRF:             s.CSRFToken,
		PasskeyRole:      "barber",
		Hairstyles:       counts.Hairstyles,
		PendingBookings:  counts.Pending,
		UpcomingBookings: counts.AcceptedUpcoming,
		Conversations:    conversations,
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
