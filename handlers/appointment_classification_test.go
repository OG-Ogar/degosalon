package handlers

import (
	"barber-booking/models"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAppointmentClassificationUsesInstantAndSnapshot(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, zone, date, clock, status string
		want                            appointmentGroup
	}{
		{"future pending", "UTC", "2026-10-07", "12:01", models.StatusPending, appointmentUpcoming},
		{"elapsed accepted", "UTC", "2026-10-07", "11:59", models.StatusAccepted, appointmentFollowUp},
		{"exact start", "UTC", "2026-10-07", "12:00:00", models.StatusPending, appointmentFollowUp},
		{"same day future in New York", "America/New_York", "2026-10-07", "09:00:00", models.StatusAccepted, appointmentUpcoming},
		{"same date past in Lagos", "Africa/Lagos", "2026-10-07", "12:30:00", models.StatusPending, appointmentFollowUp},
		{"completed future", "UTC", "2026-10-08", "09:00", models.StatusCompleted, appointmentFinished},
		{"cancelled future", "UTC", "2026-10-08", "09:00", models.StatusCancelled, appointmentFinished},
		{"rejected future", "UTC", "2026-10-08", "09:00", models.StatusRejected, appointmentFinished},
		{"invalid timezone", "invalid/timezone", "2026-10-08", "09:00", models.StatusAccepted, appointmentFollowUp},
		{"invalid time", "UTC", "2026-10-08", "bad", models.StatusPending, appointmentFollowUp},
		{"legacy future", "", "2026-10-09", "09:00", models.StatusPending, appointmentUpcoming},
	} {
		t.Run(test.name, func(t *testing.T) {
			booking := models.Booking{BookingTimezone: test.zone, Date: test.date, Time: test.clock, Status: test.status}
			if got := classifyAppointment(booking, now); got != test.want {
				t.Fatalf("group = %v, want %v", got, test.want)
			}
			if booking.Status != test.status {
				t.Fatal("classification changed lifecycle status")
			}
		})
	}
}

func TestFollowUpViewsRetainOutcomeActions(t *testing.T) {
	rows := []bookingRow{
		{Booking: models.Booking{ID: 1, BarberID: 2, CustomerID: 3, Date: "2026-10-01", Time: "10:00", Status: models.StatusAccepted, BookingTimezone: "Africa/Lagos"}, HairstyleName: "Accepted service", BarberName: "Barber"},
		{Booking: models.Booking{ID: 4, BarberID: 2, CustomerID: 3, Date: "2026-10-01", Time: "11:00", Status: models.StatusPending, BookingTimezone: "Africa/Lagos"}, HairstyleName: "Pending service", BarberName: "Barber"},
	}
	for _, test := range []struct {
		path    string
		view    any
		actions []string
	}{
		{"customer/bookings.html", customerBookingsView{FollowUp: rows, CSRF: "csrf-token", ActionURL: "/customer/bookings/"}, []string{"/customer/bookings/1/cancel", "/customer/bookings/4/cancel", "/customer/chats/2"}},
		{"barber/bookings.html", barberBookingsView{FollowUp: rows, CSRF: "csrf-token", ActionURL: "/barber/bookings/"}, []string{"/barber/bookings/1/complete", "/barber/bookings/4/reject", "/barber/chats/3"}},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			render(response, test.path, test.view)
			if response.Code != 200 {
				t.Fatalf("render = %d: %s", response.Code, response.Body.String())
			}
			for _, text := range append(test.actions, "Needs follow-up", "Accepted service", "Pending service", "csrf-token") {
				if !strings.Contains(response.Body.String(), text) {
					t.Errorf("missing %q", text)
				}
			}
		})
	}
}
