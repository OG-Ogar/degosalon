package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

// bookingRequest is a validated booking submission.
type bookingRequest struct {
	BarberID    int
	HairstyleID int
	Date        string
	Time        string
	Notes       string

	values map[string]string
}

// readBookingRequest validates the booking form.
//
// The customer is never read from the form: the caller passes the
// signed-in customer, so the IDs in the request cannot be swapped to
// book an appointment in somebody else's name.
func readBookingRequest(r *http.Request) (bookingRequest, string) {
	dateText := trimmedField(r.PostFormValue("date"))
	timeText := trimmedField(r.PostFormValue("time"))

	request := bookingRequest{
		Notes: truncateText(r.PostFormValue("notes")),
		values: map[string]string{
			"hairstyle": trimmedField(r.PostFormValue("hairstyle_id")),
			"date":      dateText,
			"time":      timeText,
			"notes":     truncateText(r.PostFormValue("notes")),
		},
	}

	barberID, ok := parseID(r.PostFormValue("barber_id"))
	if !ok {
		return request, "That barber could not be identified."
	}

	request.BarberID = barberID

	hairstyleID, ok := parseID(r.PostFormValue("hairstyle_id"))
	if !ok {
		return request, "Choose a hairstyle to book."
	}

	request.HairstyleID = hairstyleID

	if dateText == "" {
		return request, "Choose a booking date."
	}

	request.Date = dateText

	if timeText == "" {
		return request, "Choose a booking time."
	}

	request.Time = timeText

	return request, ""
}

// BookHairstyleHandler creates a booking for the signed-in customer.
//
// This is POST only. The form lives on the barber's page, and a rejected
// submission is redrawn there with the reason and the values kept.
func BookHairstyleHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	request, problem := readBookingRequest(r)

	// The barber has to resolve before anything can be redrawn, so this
	// check comes before the validation message is reported.
	barber, ok := bookableBarber(w, r.PostFormValue("barber_id"))
	if !ok {
		return
	}

	if problem != "" {
		renderBarberDetail(
			w,
			s,
			barber,
			request.HairstyleID,
			request.values,
			problem,
		)

		return
	}

	booking, err := storage.CreateBooking(models.Booking{
		BarberID:      barber.ID,
		HairstyleID:   request.HairstyleID,
		CustomerID:    customer.ID,
		CustomerName:  customer.FullName,
		CustomerPhone: customer.PhoneNumber,
		CustomerEmail: customer.Email,
		Date:          request.Date,
		Time:          request.Time,
		Notes:         request.Notes,
		Status:        models.StatusPending,
		CreatedAt:     time.Now(),
	})

	if message := bookingProblem(err); message != "" {
		renderBarberDetail(
			w,
			s,
			barber,
			request.HairstyleID,
			request.values,
			message,
		)

		return
	}

	if err != nil {
		serverError(w, "create booking", err)

		return
	}

	log.Printf(
		"booking %d: customer %d with barber %d on %s %s",
		booking.ID,
		customer.ID,
		barber.ID,
		booking.Date,
		booking.Time,
	)

	// The booking was successfully created.
	// Send the customer to the confirmation page instead of
	// immediately sending them to the bookings list.
	http.Redirect(
		w,
		r,
		"/customer/booking-confirmation/"+strconv.Itoa(booking.ID),
		http.StatusSeeOther,
	)
}

// BookingConfirmationHandler displays the successful booking request.
//
// The booking ID comes from the URL, but we still verify that the booking
// belongs to the signed-in customer before showing any information.
func BookingConfirmationHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	bookingID, ok := parseID(idSegment)
	if !ok {
		http.NotFound(w, r)

		return
	}

	booking, found := storage.FindBookingByID(bookingID)
	if !found {
		http.NotFound(w, r)

		return
	}

	// A customer must never be able to view another customer's booking
	// simply by changing the booking ID in the URL.
	if booking.CustomerID != customer.ID {
		http.NotFound(w, r)

		return
	}

	barber, found := storage.FindBarberByID(booking.BarberID)
	if !found {
		http.NotFound(w, r)

		return
	}

	hairstyle, found := storage.FindHairstyleByID(booking.HairstyleID)
	if !found {
		http.NotFound(w, r)

		return
	}

	view := bookingConfirmationView{
		CSRF:      s.CSRFToken,
		Booking:   booking,
		Barber:    barber,
		Hairstyle: hairstyle,
	}

	render(w, "customer/booking-confirmation.html", view)
}

type bookingConfirmationView struct {
	CSRF      string
	Booking   models.Booking
	Barber    models.Barber
	Hairstyle models.Hairstyle
}

// bookingProblem turns the storage errors a customer can actually cause
// into a message for the form, and returns "" for anything else so the
// caller reports it as a server error instead.
func bookingProblem(err error) string {
	switch {
	case err == nil:
		return ""

	case errors.Is(err, storage.ErrSlotTaken):
		return "That time is already taken. Please choose another slot."

	case errors.Is(err, storage.ErrUnavailable):
		return "That hairstyle is not available for booking right now."

	case errors.Is(err, storage.ErrNotFound),
		errors.Is(err, storage.ErrForbidden):
		return "That hairstyle does not belong to this barber."
	}

	return ""
}
