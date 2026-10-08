package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"time"
)

type appointmentGroup = storage.AppointmentGroup

const (
	appointmentFinished = storage.AppointmentFinished
	appointmentUpcoming = storage.AppointmentUpcoming
	appointmentFollowUp = storage.AppointmentFollowUp
)

func classifyAppointment(booking models.Booking, now time.Time) appointmentGroup {
	return storage.ClassifyAppointment(booking, now, BookingTimezone())
}
