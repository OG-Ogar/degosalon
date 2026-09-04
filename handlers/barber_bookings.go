package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberBookingsView splits the schedule into the three groups a barber
// actually acts on, so the page does not need template logic to filter.
type barberBookingsView struct {
	Pending   []bookingRow
	Upcoming  []bookingRow
	Finished  []bookingRow
	CSRF      string
	ActionURL string
}

// BarberBookingsHandler lists the requests and appointments for the
// signed-in barber.
func BarberBookingsHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	view := barberBookingsView{
		CSRF:      s.CSRFToken,
		ActionURL: "/barber/bookings/",
	}

	for _, booking := range storage.ListBookingsByBarber(barber.ID) {
		row := describeBooking(booking)

		switch booking.Status {
		case models.StatusPending:
			view.Pending = append(view.Pending, row)
		case models.StatusAccepted:
			view.Upcoming = append(view.Upcoming, row)
		default:
			view.Finished = append(view.Finished, row)
		}
	}

	render(w, "barber/bookings.html", view)
}

// barberBookingActions maps a URL action to the status it sets and the
// statuses it is allowed to move from. Restricting the source status is
// what stops a barber from re-opening a booking the customer cancelled.
var barberBookingActions = map[string]struct {
	status string
	from   []string
}{
	"accept": {
		status: models.StatusAccepted,
		from:   []string{models.StatusPending},
	},
	"reject": {
		status: models.StatusRejected,
		from:   []string{models.StatusPending},
	},
	"complete": {
		status: models.StatusCompleted,
		from:   []string{models.StatusAccepted},
	},
}

// BarberBookingActionHandler applies accept, reject or complete to one
// booking on the signed-in barber's schedule.
func BarberBookingActionHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
	action string,
) {
	change, known := barberBookingActions[action]
	if !known {
		http.NotFound(w, r)

		return
	}

	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	id, ok := parseID(idSegment)
	if !ok {
		http.Error(w, "Booking not found.", http.StatusNotFound)

		return
	}

	booking, found := storage.FindBookingByID(id)
	if !found || booking.BarberID != barber.ID {
		http.Error(w, "Booking not found.", http.StatusNotFound)

		return
	}

	if !containsStatus(change.from, booking.Status) {
		http.Error(
			w,
			"That booking is "+booking.Status+
				", so it cannot be changed to "+change.status+".",
			http.StatusConflict,
		)

		return
	}

	err := storage.SetBookingStatus(id, change.status, barber.ID, 0)

	switch {
	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		http.Error(w, "Booking not found.", http.StatusNotFound)

		return

	case err != nil:
		serverError(w, "set booking status", err)

		return
	}

	http.Redirect(w, r, "/barber/bookings", http.StatusSeeOther)
}

// containsStatus reports whether status is in the allowed list.
func containsStatus(allowed []string, status string) bool {
	for _, candidate := range allowed {
		if candidate == status {
			return true
		}
	}

	return false
}
