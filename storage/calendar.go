package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"barber-booking/models"
)

var ErrOutsideSchedule = errors.New("appointment is outside the available schedule")
var ErrScheduleConflict = errors.New("calendar change would invalidate an existing appointment")
var calendars = map[int]models.Calendar{}
var calendarDefault = models.Calendar{Timezone: "Africa/Lagos", LeadMinutes: 30, HorizonDays: 365, Week: [7]models.DayHours{
	{Open: "08:00", Close: "20:00"}, {Open: "08:00", Close: "20:00"}, {Open: "08:00", Close: "20:00"}, {Open: "08:00", Close: "20:00"}, {Open: "08:00", Close: "20:00"}, {Open: "08:00", Close: "20:00"}, {Open: "08:00", Close: "20:00"},
}}

// ConfigureCalendarDefaults applies the configured business fallback at startup.
func ConfigureCalendarDefaults(zone, open, close string) {
	calendarDefault.Timezone = zone
	for i := range calendarDefault.Week {
		calendarDefault.Week[i] = models.DayHours{Open: open, Close: close}
	}
}

func minute(value string) (int, error) {
	if value == "24:00" {
		return 1440, nil
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func ValidateCalendar(c models.Calendar) error {
	if _, err := time.LoadLocation(c.Timezone); err != nil || c.Timezone == "" || c.Timezone == "Local" {
		return fmt.Errorf("Choose a valid timezone, such as Africa/Lagos.")
	}
	if c.LeadMinutes < 0 || c.LeadMinutes > 10080 || c.HorizonDays < 1 || c.HorizonDays > 365 {
		return fmt.Errorf("Lead time must be 0–10080 minutes and advance booking 1–365 days.")
	}
	for _, day := range c.Week {
		if day.Closed {
			continue
		}
		open, e1 := minute(day.Open)
		close, e2 := minute(day.Close)
		if e1 != nil || e2 != nil || open >= close {
			return fmt.Errorf("Each open day needs an opening time before closing time.")
		}
		if day.BreakStart != "" || day.BreakEnd != "" {
			start, e1 := minute(day.BreakStart)
			end, e2 := minute(day.BreakEnd)
			if e1 != nil || e2 != nil || start < open || end > close || start >= end {
				return fmt.Errorf("Breaks must start and end within opening hours.")
			}
		}
	}
	if len(c.TimeOff) > 60 {
		return fmt.Errorf("Keep at most 60 time-off periods.")
	}
	for _, off := range c.TimeOff {
		_, dateErr := time.Parse("2006-01-02", off.Date)
		start, e1 := minute(off.Start)
		end, e2 := minute(off.End)
		if dateErr != nil || e1 != nil || e2 != nil || start >= end {
			return fmt.Errorf("Time off needs a valid date and a start time before its end.")
		}
	}
	return nil
}

func calendarAllows(c models.Calendar, date, at string, duration int, now time.Time, enforceLead bool) bool {
	location, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return false
	}
	start, err := time.ParseInLocation("2006-01-02 15:04", date+" "+at, location)
	if err != nil || duration <= 0 || start.Format("2006-01-02 15:04") != date+" "+at {
		return false
	}
	if enforceLead {
		if !start.After(now.Add(time.Duration(c.LeadMinutes)*time.Minute)) || date > now.In(location).AddDate(0, 0, c.HorizonDays).Format("2006-01-02") {
			return false
		}
	}
	day := c.Week[start.Weekday()]
	open, e1 := minute(day.Open)
	close, e2 := minute(day.Close)
	from := start.Hour()*60 + start.Minute()
	until := from + duration
	if day.Closed || e1 != nil || e2 != nil || from < open || until > close {
		return false
	}
	if day.BreakStart != "" {
		a, _ := minute(day.BreakStart)
		b, _ := minute(day.BreakEnd)
		if from < b && a < until {
			return false
		}
	}
	for _, off := range c.TimeOff {
		if off.Date == date {
			a, _ := minute(off.Start)
			b, _ := minute(off.End)
			if from < b && a < until {
				return false
			}
		}
	}
	return true
}

type calendarReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readCalendar(ctx context.Context, conn calendarReader, id int) (models.Calendar, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	var data []byte
	err := conn.QueryRowContext(ctx, "select settings from public.barber_calendars where barber_id=$1", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return calendarDefault, nil
	}
	if err != nil {
		return models.Calendar{}, err
	}
	var c models.Calendar
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, ValidateCalendar(c)
}

func CalendarForBarber(ctx context.Context, id int) (models.Calendar, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return models.Calendar{}, err
	}
	if databaseEnabled() {
		if _, err := FindBarberByIDContext(ctx, id); err != nil {
			return models.Calendar{}, err
		}
		return readCalendar(ctx, db, id)
	}
	mu.RLock()
	defer mu.RUnlock()
	if _, found := findBarberByID(id); !found {
		return models.Calendar{}, ErrNotFound
	}
	c, found := calendars[id]
	if !found {
		c = calendarDefault
	}
	c.TimeOff = append([]models.TimeOff(nil), c.TimeOff...)
	return c, nil
}

func SaveBarberCalendar(ctx context.Context, id int, c models.Calendar) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateCalendar(c); err != nil {
		return err
	}
	now := time.Now()
	location, _ := time.LoadLocation(c.Timezone)
	date := now.In(location).AddDate(0, 0, -1).Format("2006-01-02")
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, "select pg_advisory_xact_lock($1)", id); err != nil {
			return err
		}
		var active bool
		if err := tx.QueryRowContext(ctx, "select status='Active' from public.barbers where id=$1 for update", id).Scan(&active); err != nil {
			return recordError(err)
		}
		if !active {
			return ErrUnavailable
		}
		rows, err := tx.QueryContext(ctx, "select booking_date::text,to_char(booking_time,'HH24:MI'),service_duration,coalesce(booking_timezone,'') from public.bookings where barber_id=$1 and booking_date >= $2 and status in ('Pending','Accepted','Completed')", id, date)
		if err != nil {
			return err
		}
		for rows.Next() {
			var day, at string
			var duration int
			var zone string
			if err := rows.Scan(&day, &at, &duration, &zone); err != nil {
				rows.Close()
				return err
			}
			originalZone := zone
			if originalZone == "" {
				originalZone = calendarDefault.Timezone
			}
			originalLocation, e := time.LoadLocation(originalZone)
			if e != nil {
				rows.Close()
				return e
			}
			start, e := time.ParseInLocation("2006-01-02 15:04", day+" "+at, originalLocation)
			if e != nil {
				rows.Close()
				return e
			}
			if !start.After(now) {
				continue
			}
			if (zone != "" && zone != c.Timezone) || !calendarAllows(c, day, at, duration, now, false) {
				rows.Close()
				return ErrScheduleConflict
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		data, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "insert into public.barber_calendars(barber_id,settings) values($1,$2) on conflict(barber_id) do update set settings=excluded.settings", id, string(data)); err != nil {
			return err
		}
		return tx.Commit()
	}
	mu.Lock()
	defer mu.Unlock()
	barber, found := findBarberByID(id)
	if !found {
		return ErrNotFound
	}
	if !barber.IsActive() {
		return ErrUnavailable
	}
	for _, booking := range bookings {
		if booking.BarberID == id && booking.IsOpen() && booking.Date >= date {
			duration := booking.ServiceDuration
			if duration == 0 {
				if style, ok := findHairstyleByID(booking.HairstyleID); ok {
					duration = style.Duration
				}
			}
			at := booking.Time
			if len(at) > 5 {
				at = at[:5]
			}
			originalZone := booking.BookingTimezone
			if originalZone == "" {
				originalZone = calendarDefault.Timezone
			}
			originalLocation, e := time.LoadLocation(originalZone)
			if e != nil {
				return e
			}
			start, e := time.ParseInLocation("2006-01-02 15:04", booking.Date+" "+at, originalLocation)
			if e != nil {
				return e
			}
			if !start.After(now) {
				continue
			}
			if (booking.BookingTimezone != "" && booking.BookingTimezone != c.Timezone) || !calendarAllows(c, booking.Date, at, duration, now, false) {
				return ErrScheduleConflict
			}
		}
	}
	c.TimeOff = append([]models.TimeOff(nil), c.TimeOff...)
	calendars[id] = c
	return nil
}

func AvailableSlots(ctx context.Context, barberID, styleID int, date string) ([]string, models.Calendar, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, models.Calendar{}, ErrOutsideSchedule
	}
	c, err := CalendarForBarber(ctx, barberID)
	if err != nil {
		return nil, c, err
	}
	barber, err := FindBarberByIDContext(ctx, barberID)
	if err != nil {
		return nil, c, err
	}
	if !barber.IsActive() {
		return nil, c, ErrUnavailable
	}
	style, err := FindHairstyleOwnedBy(ctx, styleID, barberID)
	if err != nil {
		return nil, c, err
	}
	if !style.IsAvailable {
		return nil, c, ErrUnavailable
	}
	existing, err := availabilityBookings(ctx, barberID, date)
	if err != nil {
		return nil, c, err
	}
	now := time.Now()
	slots := []string{}
	for at := 0; at < 1440; at += 15 {
		label := fmt.Sprintf("%02d:%02d", at/60, at%60)
		if !calendarAllows(c, date, label, style.Duration, now, true) {
			continue
		}
		free := true
		for _, booking := range existing {
			if booking.Date == date && booking.IsOpen() && bookingIntervalsOverlap(label, style.Duration, booking.Time, booking.ServiceDuration) {
				free = false
				break
			}
		}
		if free {
			slots = append(slots, label)
		}
	}
	return slots, c, nil
}

func availabilityBookings(ctx context.Context, id int, date string) ([]models.Booking, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if databaseEnabled() {
		return queryRecords(ctx, `select to_char(booking_time,'HH24:MI'),service_duration from public.bookings where barber_id=$1 and booking_date=$2 and status in ('Pending','Accepted','Completed')`, func(rows *sql.Rows) (models.Booking, error) {
			v := models.Booking{BarberID: id, Date: date, Status: models.StatusPending}
			err := rows.Scan(&v.Time, &v.ServiceDuration)
			return v, err
		}, id, date)
	}
	mu.RLock()
	defer mu.RUnlock()
	var result []models.Booking
	for _, booking := range bookings {
		if booking.BarberID == id && booking.Date == date && booking.IsOpen() {
			result = append(result, booking)
		}
	}
	return result, nil
}
