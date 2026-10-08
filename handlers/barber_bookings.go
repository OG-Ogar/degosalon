package handlers

import (
	"errors"
	"net/http"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberBookingsView splits the schedule into the three groups a barber
// actually acts on, so the page does not need template logic to filter.
type barberBookingsView struct {
	Pending    []bookingRow
	Upcoming   []bookingRow
	Finished   []bookingRow
	FollowUp   []bookingRow
	CSRF       string
	ActionURL  string
	Pagination listPagination
	Group      string
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

	cursor, ok := readPageCursor(w, r)
	if !ok {
		return
	}
	group, ok := readBookingGroup(w, r)
	if !ok {
		return
	}
	view.Group = group
	now := time.Now()
	filter := storage.BookingFilter{BarberID: barber.ID, Group: group, Now: now, LegacyTimezone: BookingTimezone()}
	page, err := storage.BookingPage(r.Context(), filter, cursor)
	if err != nil {
		serverError(w, "load barber booking page", err)
		return
	}
	view.Pagination = bookingPagination(r, cursor, page)
	if len(page.Items) == 0 && (cursor.After > 0 || cursor.Before > 0) {
		http.Redirect(w, r, view.Pagination.FirstLink, 303)
		return
	}
	for _, row := range page.Items {
		booking := row.Booking

		switch classifyAppointment(booking, now) {
		case appointmentFollowUp:
			view.FollowUp = append(view.FollowUp, row)
		case appointmentUpcoming:
			if booking.Status == models.StatusPending {
				view.Pending = append(view.Pending, row)
			} else {
				view.Upcoming = append(view.Upcoming, row)
			}
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
		journeyError(w, "Page not found.", http.StatusNotFound)

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
		journeyError(w, "Booking not found.", http.StatusNotFound)

		return
	}

	booking, err := storage.FindBookingByIDResult(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load booking action", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) || booking.BarberID != barber.ID {
		journeyError(w, "Booking not found.", http.StatusNotFound)

		return
	}

	if !containsStatus(change.from, booking.Status) {
		journeyError(
			w,
			"That booking is "+booking.Status+
				", so it cannot be changed to "+change.status+".",
			http.StatusConflict,
		)

		return
	}

	err = storage.TransitionBookingStatusContext(r.Context(), id, change.status, change.from, barber.ID, 0)

	switch {
	case errors.Is(err, storage.ErrInvalidTransition):
		journeyError(w, "That booking changed before this action completed. Refresh the schedule and try again.", http.StatusConflict)
		return

	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		journeyError(w, "Booking not found.", http.StatusNotFound)

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
