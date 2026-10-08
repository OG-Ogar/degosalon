package handlers

import (
	"errors"
	"net/http"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

// customerBookingsView splits a customer's appointments the way they
// think about them: what is still coming, and what is done with.
type customerBookingsView struct {
	Upcoming []bookingRow
	Past     []bookingRow
	FollowUp []bookingRow

	CSRF       string
	ActionURL  string
	Pagination listPagination
	Group      string
}

// CustomerBookingsHandler lists the signed-in customer's appointments.
func CustomerBookingsHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	view := customerBookingsView{
		CSRF:      s.CSRFToken,
		ActionURL: "/customer/bookings/",
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
	filter := storage.BookingFilter{CustomerID: customer.ID, Group: group, Now: now, LegacyTimezone: BookingTimezone()}
	page, err := storage.BookingPage(r.Context(), filter, cursor)
	if err != nil {
		serverError(w, "load customer booking page", err)
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
		case appointmentUpcoming:
			view.Upcoming = append(view.Upcoming, row)
		case appointmentFollowUp:
			view.FollowUp = append(view.FollowUp, row)
		default:
			view.Past = append(view.Past, row)
		}
	}

	render(w, "customer/bookings.html", view)
}

// CancelBookingHandler lets a customer withdraw their own booking.
//
// The customer ID is passed to storage so the ownership check happens in
// the same lock as the write; a guessed booking ID belonging to somebody
// else is refused rather than cancelled.
func CancelBookingHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	idSegment string,
) {
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
	if errors.Is(err, storage.ErrNotFound) || booking.CustomerID != customer.ID {
		journeyError(w, "Booking not found.", http.StatusNotFound)

		return
	}

	if !booking.CanBeCancelled() {
		journeyError(
			w,
			"That booking is "+booking.Status+" and can no longer be cancelled.",
			http.StatusConflict,
		)

		return
	}

	err = storage.TransitionBookingStatusContext(r.Context(), id, models.StatusCancelled, []string{models.StatusPending, models.StatusAccepted}, 0, customer.ID)

	switch {
	case errors.Is(err, storage.ErrInvalidTransition):
		journeyError(w, "That booking changed before cancellation completed. Refresh your bookings and try again.", http.StatusConflict)
		return

	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		journeyError(w, "Booking not found.", http.StatusNotFound)

		return

	case err != nil:
		serverError(w, "cancel booking", err)

		return
	}

	http.Redirect(w, r, "/customer/bookings", http.StatusSeeOther)
}
