package storage

import (
	"barber-booking/models"
	"strings"
	"time"
)

type AppointmentGroup int

const (
	AppointmentFinished AppointmentGroup = iota
	AppointmentUpcoming
	AppointmentFollowUp
)

func ClassifyAppointment(booking models.Booking, now time.Time, legacyTimezone string) AppointmentGroup {
	if !booking.CanBeCancelled() {
		return AppointmentFinished
	}
	zone := booking.BookingTimezone
	if zone == "" {
		zone = legacyTimezone
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return AppointmentFollowUp
	}
	clock := strings.TrimSpace(booking.Time)
	if len(clock) == 5 {
		clock += ":00"
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", booking.Date+" "+clock, location)
	if err != nil || !start.After(now) {
		return AppointmentFollowUp
	}
	return AppointmentUpcoming
}
