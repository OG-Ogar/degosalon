package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// customerBookingsView splits a customer's appointments the way they
// think about them: what is still coming, and what is done with.
type customerBookingsView struct {
	Upcoming []bookingRow
	Past     []bookingRow

	CSRF      string
	ActionURL string
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

	bookings, err := storage.ListBookingsByCustomerResult(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load customer bookings", err)
		return
	}
	for _, booking := range bookings {
		row, err := describeBooking(r.Context(), booking)
		if err != nil {
			serverError(w, "load appointment details", err)
			return
		}

		if booking.CanBeCancelled() {
			view.Upcoming = append(view.Upcoming, row)
		} else {
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
		http.Error(w, "Booking not found.", http.StatusNotFound)

		return
	}

	booking, err := storage.FindBookingByIDResult(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load booking action", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) || booking.CustomerID != customer.ID {
		http.Error(w, "Booking not found.", http.StatusNotFound)

		return
	}

	if !booking.CanBeCancelled() {
		http.Error(
			w,
			"That booking is "+booking.Status+" and can no longer be cancelled.",
			http.StatusConflict,
		)

		return
	}

	err = storage.TransitionBookingStatus(id, models.StatusCancelled, []string{models.StatusPending, models.StatusAccepted}, 0, customer.ID)

	switch {
	case errors.Is(err, storage.ErrInvalidTransition):
		http.Error(w, "That booking changed before cancellation completed. Refresh your bookings and try again.", http.StatusConflict)
		return

	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		http.Error(w, "Booking not found.", http.StatusNotFound)

		return

	case err != nil:
		serverError(w, "cancel booking", err)

		return
	}

	http.Redirect(w, r, "/customer/bookings", http.StatusSeeOther)
}
