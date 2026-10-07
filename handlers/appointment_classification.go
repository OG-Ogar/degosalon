package handlers

import (
	"barber-booking/models"
	"strings"
	"time"
)

type appointmentGroup int

const (
	appointmentFinished appointmentGroup = iota
	appointmentUpcoming
	appointmentFollowUp
)

// Elapsed open appointments require an explicit outcome. Reading a page must
// never infer a no-show or change the persisted booking lifecycle.
func classifyAppointment(booking models.Booking, now time.Time) appointmentGroup {
	if !booking.CanBeCancelled() {
		return appointmentFinished
	}
	location := bookingLocation // Legacy records without a snapshot use the configured legacy timezone.
	if booking.BookingTimezone != "" {
		var err error
		location, err = time.LoadLocation(booking.BookingTimezone)
		if err != nil {
			return appointmentFollowUp
		}
	}
	// PostgreSQL time::text includes seconds; older memory rows use HH:MM.
	clock := strings.TrimSpace(booking.Time)
	if len(clock) == 5 {
		clock += ":00"
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", booking.Date+" "+clock, location)
	if err != nil || !start.After(now) {
		return appointmentFollowUp
	}
	return appointmentUpcoming
}
