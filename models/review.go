package models

import "time"

// Review is customer feedback for a barber after a completed appointment.
type Review struct {
	ID int
	BarberID int
	CustomerID int
	BookingID int
	CustomerName string
	Rating int
	Comment string
	CreatedAt time.Time
}
