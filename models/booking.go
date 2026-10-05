package models

import "time"

// Booking lifecycle states.
//
// A customer creates a booking as StatusPending. The barber then moves
// it to StatusAccepted or StatusRejected, and finally to
// StatusCompleted once the haircut is done. The customer may move
// their own pending or accepted booking to StatusCancelled.
const (
	StatusPending   = "Pending"
	StatusAccepted  = "Accepted"
	StatusRejected  = "Rejected"
	StatusCancelled = "Cancelled"
	StatusCompleted = "Completed"
)

// Booking is one appointment for one hairstyle with one barber.
type Booking struct {
	ID          int
	BarberID    int
	HairstyleID int
	CustomerID  int

	// Service/shop metadata is frozen when the appointment is created.
	BarberName      string
	ShopName        string
	ShopAddress     string
	ServiceName     string
	ServicePrice    float64
	ServiceDuration int
	BookingTimezone string

	// Contact details are copied from the customer when the booking is
	// made, so the barber keeps a record even if the customer later
	// edits their profile.
	CustomerName  string
	CustomerPhone string
	CustomerEmail string

	// Date is YYYY-MM-DD and Time is HH:MM, both as entered by the
	// customer and validated on the way in.
	Date string
	Time string

	Notes  string
	Status string

	CreatedAt time.Time
}

// IsOpen reports whether the booking still occupies its time slot.
// Rejected and cancelled bookings free the slot up again.
func (b Booking) IsOpen() bool {
	return b.Status == StatusPending ||
		b.Status == StatusAccepted ||
		b.Status == StatusCompleted
}

// CanBeCancelled reports whether a customer may still cancel.
func (b Booking) CanBeCancelled() bool {
	return b.Status == StatusPending || b.Status == StatusAccepted
}
