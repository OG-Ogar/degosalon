package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"context"
	"errors"
)

// formView backs every page that renders a form. Values holds what the
// visitor typed so a rejected submission comes back filled in instead
// of blank, and Error carries the reason.
//
// Notice is the opposite: something that went right elsewhere and is
// worth saying on arrival, such as a password that has just been reset.
type formView struct {
	CSRF   string
	Error  string
	Notice string
	Values map[string]string
}

// newFormView starts an empty form.
func newFormView(csrf string) formView {
	return formView{
		CSRF:   csrf,
		Values: map[string]string{},
	}
}

// barberView renders one barber plus the CSRF token. The embedded
// struct keeps templates written as {{.FullName}} working unchanged.
type barberView struct {
	models.Barber

	CSRF  string
	Error string
}

// customerView renders one customer plus the CSRF token.
type customerView struct {
	models.Customer

	CSRF  string
	Error string
}

// hairstyleView renders one hairstyle plus the CSRF token.
type hairstyleView struct {
	models.Hairstyle

	CSRF  string
	Error string
}

// listView renders a table or card grid of anything.
type listView[T any] struct {
	Query      string
	Pagination listPagination
	Items      []T
	CSRF       string
	Error      string
}

// bookingRow is one line in a bookings table, with the names and
// hairstyle details resolved from the IDs stored by the booking.
//
// The customer's name and phone are not resolved here: Booking copies
// them at creation time so the barber keeps the details that were given
// when the appointment was made.
type bookingRow = storage.BookingCard

// describeBooking fills in the barber and hairstyle details for display.
func describeBooking(ctx context.Context, booking models.Booking) (bookingRow, error) {
	if err := ctx.Err(); err != nil {
		return bookingRow{}, err
	}
	if booking.ServiceDuration > 0 {
		return bookingRow{Booking: booking, BarberName: booking.BarberName, ShopName: booking.ShopName, HairstyleName: booking.ServiceName, HairstyleImage: booking.ServiceImage, Price: booking.ServicePrice, Duration: booking.ServiceDuration}, nil
	}
	row := bookingRow{
		Booking: booking,
	}

	barber, err := storage.FindBarberByIDContext(ctx, booking.BarberID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return bookingRow{}, err
	}
	if err == nil {
		row.BarberName = barber.FullName
		row.ShopName = barber.ShopName
	}

	hairstyle, err := storage.FindHairstyleByIDResult(ctx, booking.HairstyleID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return bookingRow{}, err
	}
	if err == nil {
		row.HairstyleName = hairstyle.Name
		row.HairstyleImage = hairstyle.Image
		row.Price = hairstyle.Price
		row.Duration = hairstyle.Duration
	}

	return row, nil
}

// describeBookings maps describeBooking over a slice.
func describeBookings(ctx context.Context, bookings []models.Booking) ([]bookingRow, error) {
	rows := make([]bookingRow, 0, len(bookings))

	for _, booking := range bookings {
		row, err := describeBooking(ctx, booking)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// chatRow is one entry in a list of conversations. Name is the person on
// the other side and Detail is whatever identifies them best: the shop
// name when a customer looks at barbers, the phone number when a barber
// looks at customers.
//
// CustomerImage is used when the barber is viewing customer conversations.
// BarberImage is used when the customer is viewing barber conversations.
type chatRow struct {
	BarberID   int
	CustomerID int

	Name          string
	Detail        string
	CustomerImage string
	BarberImage   string

	LastText string
	LastAt   string
	Count    int
}

// chatView is one open conversation thread.
type chatView struct {
	Pagination listPagination
	Messages   []models.Message

	// Title names the person on the other side of the chat.
	Title       string
	Subtitle    string
	BarberImage string

	// BackLink returns to the dashboard of whichever role is viewing.
	BackLink string
	Action   string

	CSRF  string
	Error string
}
