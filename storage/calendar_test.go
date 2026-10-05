package storage

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"barber-booking/models"
)

func TestCalendarAvailability(t *testing.T) { Reset(); t.Cleanup(Reset); checkCalendarAvailability(t) }

func checkCalendarAvailability(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	barber := newBarber(t, "calendar_barber@example.com")
	customer := newCustomer(t, "calendar_buyer@example.com")
	style := newHairstyle(t, barber.ID)
	style.Duration = 60
	if err := UpdateHairstyle(style); err != nil {
		t.Fatal(err)
	}
	c, err := CalendarForBarber(ctx, barber.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.Timezone = "Africa/Lagos"
	c.LeadMinutes = 60
	c.HorizonDays = 30
	for i := range c.Week {
		c.Week[i] = models.DayHours{Open: "09:00", Close: "17:00", BreakStart: "12:00", BreakEnd: "13:00"}
	}
	date := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	c.TimeOff = []models.TimeOff{{Date: date, Start: "14:00", End: "15:00"}}
	if err := SaveBarberCalendar(ctx, barber.ID, c); err != nil {
		t.Fatal(err)
	}
	slots, _, err := AvailableSlots(ctx, barber.ID, style.ID, date)
	if err != nil {
		t.Fatal(err)
	}
	for _, unavailable := range []string{"08:45", "11:15", "12:00", "13:15", "14:00", "16:15"} {
		if slices.Contains(slots, unavailable) {
			t.Fatalf("unavailable slot %s offered", unavailable)
		}
	}
	for _, available := range []string{"09:00", "11:00", "13:00", "15:00", "16:00"} {
		if !slices.Contains(slots, available) {
			t.Fatalf("available slot %s missing", available)
		}
	}
	request := newBooking(customer, barber.ID, style.ID)
	request.Date = date
	request.Time = "09:00"
	booking, err := CreateBookingContext(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if booking.BookingTimezone != c.Timezone {
		t.Fatal("booking timezone not recorded")
	}
	slots, _, err = AvailableSlots(ctx, barber.ID, style.ID, date)
	if err != nil || slices.Contains(slots, "09:30") {
		t.Fatalf("occupied slot offered: %v %v", slots, err)
	}
	for _, at := range []string{"08:00", "11:30", "12:00", "14:00", "16:30"} {
		request.Time = at
		if _, err := CreateBookingContext(ctx, request); !errors.Is(err, ErrOutsideSchedule) {
			t.Fatalf("outside policy %s: %v", at, err)
		}
	}
	changed := c
	changed.TimeOff = append(append([]models.TimeOff(nil), c.TimeOff...), models.TimeOff{Date: date, Start: "09:00", End: "10:00"})
	if err := SaveBarberCalendar(ctx, barber.ID, changed); !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("appointment invalidated by time off: %v", err)
	}
	changed = c
	changed.Timezone = "Europe/London"
	if err := SaveBarberCalendar(ctx, barber.ID, changed); !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("appointment shifted by timezone: %v", err)
	}
	far, _, _ := AvailableSlots(ctx, barber.ID, style.ID, time.Now().AddDate(0, 0, 40).Format("2006-01-02"))
	if len(far) != 0 {
		t.Fatal("horizon ignored")
	}
	otherBarber := newBarber(t, "calendar_race@example.com")
	otherStyle := newHairstyle(t, otherBarber.ID)
	raceCalendar := c
	raceCalendar.TimeOff = []models.TimeOff{{Date: date, Start: "10:00", End: "11:00"}}
	raceBooking := newBooking(customer, otherBarber.ID, otherStyle.ID)
	raceBooking.Date = date
	raceBooking.Time = "10:00"
	outcomes := concurrentErrors(func() error { return SaveBarberCalendar(ctx, otherBarber.ID, raceCalendar) }, func() error { _, err := CreateBookingContext(ctx, raceBooking); return err })
	successes := 0
	for _, err := range outcomes {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrOutsideSchedule) && !errors.Is(err, ErrScheduleConflict) {
			t.Fatalf("calendar race: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("incompatible calendar and appointment both committed: %v", outcomes)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := SaveBarberCalendar(canceled, barber.ID, c); !errors.Is(err, context.Canceled) {
		t.Fatalf("calendar cancellation: %v", err)
	}
}

func TestCalendarLeadTimeAndClosedDays(t *testing.T) {
	c := calendarDefault
	c.Timezone = "Africa/Lagos"
	c.LeadMinutes = 60
	location, _ := time.LoadLocation(c.Timezone)
	now := time.Date(2030, 1, 7, 9, 30, 0, 0, location)
	if calendarAllows(c, "2030-01-07", "10:00", 30, now, true) {
		t.Fatal("lead time ignored")
	}
	c.Week[time.Monday] = models.DayHours{Closed: true}
	if calendarAllows(c, "2030-01-07", "11:00", 30, now, true) {
		t.Fatal("closed day accepted")
	}
	c.Timezone = "America/New_York"
	c.Week[time.Sunday] = models.DayHours{Open: "00:00", Close: "20:00"}
	if calendarAllows(c, "2030-03-10", "02:30", 30, time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC), true) {
		t.Fatal("nonexistent DST time accepted")
	}
}
