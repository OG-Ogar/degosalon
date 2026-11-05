package handlers

import (
	"net/http"

	"barber-booking/models"
)

// barberDashboardView is the barber's landing page: their name plus the
// counts that tell them whether anything needs attention.
type barberDashboardView struct {
	models.Barber
	Feed socialFeedView
	CSRF string
}

// BarberDashboardHandler shows the barber's own overview.
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

	// The password hash must never reach the template.
	barber.PasswordHash = ""
	feed, err := loadSocialFeed(r, models.SenderBarber, barber.ID, barber.FullName, actorPhoto, s.CSRFToken)
	if err != nil {
		serverError(w, "load social home feed", err)
		return
	}
	render(w, "barber/dashboard.html", barberDashboardView{Barber: barber, Feed: feed, CSRF: s.CSRFToken})
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
