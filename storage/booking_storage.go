package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"barber-booking/models"
)

// ErrUnavailable is returned when the requested hairstyle exists but is
// not open for booking, or the barber is suspended.
var ErrUnavailable = errors.New("that hairstyle is not available for booking")

// ErrSlotConflict is returned when the requested appointment overlaps an
// existing booking once service duration is taken into account.
var ErrSlotConflict = errors.New("that appointment overlaps an existing booking")

// ErrInvalidTransition is returned when a booking changed after it was read
// or when the requested lifecycle move is not allowed from its current state.
var ErrInvalidTransition = errors.New("booking status changed")

// CreateBooking validates and stores a new booking.
//
// The barber, hairstyle, availability and slot-conflict checks all run
// inside one write lock, so two customers submitting the same slot at
// the same moment cannot both be accepted.
func CreateBooking(booking models.Booking) (models.Booking, error) {
	return CreateBookingContext(context.Background(), booking)
}

func CreateBookingContext(ctx context.Context, booking models.Booking) (models.Booking, error) {
	if err := ctx.Err(); err != nil {
		return models.Booking{}, err
	}
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return models.Booking{}, err
		}
		defer tx.Rollback()

		// Serialize booking decisions for one barber so the overlap check and
		// insert are one atomic decision even when two requests arrive together.
		if _, err = tx.ExecContext(ctx, "select pg_advisory_xact_lock($1)", booking.BarberID); err != nil {
			return models.Booking{}, err
		}

		var active bool
		if err = tx.QueryRowContext(ctx, "select status='Active',full_name,shop_name,shop_address from public.barbers where id=$1 for share", booking.BarberID).Scan(&active, &booking.BarberName, &booking.ShopName, &booking.ShopAddress); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return models.Booking{}, ErrNotFound
			}
			return models.Booking{}, err
		}
		if !active {
			return models.Booking{}, ErrUnavailable
		}

		var available bool
		var owner, duration int
		if err = tx.QueryRowContext(ctx, "select barber_id,is_available,duration,name,price from public.hairstyles where id=$1 for share", booking.HairstyleID).Scan(&owner, &available, &duration, &booking.ServiceName, &booking.ServicePrice); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return models.Booking{}, ErrNotFound
			}
			return models.Booking{}, err
		}
		if owner != booking.BarberID {
			return models.Booking{}, ErrForbidden
		}
		if !available {
			return models.Booking{}, ErrUnavailable
		}

		calendar, err := readCalendar(ctx, tx, booking.BarberID)
		if err != nil {
			return models.Booking{}, err
		}
		if !calendarAllows(calendar, booking.Date, booking.Time, duration, time.Now(), true) {
			return models.Booking{}, ErrOutsideSchedule
		}
		booking.ServiceDuration = duration
		booking.BookingTimezone = calendar.Timezone
		var conflict bool
		err = tx.QueryRowContext(ctx, `
			select exists (
				select 1
				from public.bookings b
				where b.barber_id=$1 and b.booking_date=$2
				  and b.status in ('Pending','Accepted','Completed')
				  and b.booking_time < $3::time + ($4 * interval '1 minute')
				  and $3::time < b.booking_time + (b.service_duration * interval '1 minute')
			)`,
			booking.BarberID, booking.Date, booking.Time, duration,
		).Scan(&conflict)
		if err != nil {
			return models.Booking{}, err
		}
		if conflict {
			return models.Booking{}, ErrSlotConflict
		}

		err = tx.QueryRowContext(ctx, "insert into public.bookings(barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date,booking_time,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,booking_timezone) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) returning id", booking.BarberID, booking.HairstyleID, booking.CustomerID, booking.CustomerName, booking.CustomerPhone, booking.CustomerEmail, booking.Date, booking.Time, booking.Notes, booking.Status, booking.CreatedAt, booking.BarberName, booking.ShopName, booking.ShopAddress, booking.ServiceName, booking.ServicePrice, booking.ServiceDuration, booking.BookingTimezone).Scan(&booking.ID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "bookings_open_slot_unique") {
				return models.Booking{}, ErrSlotTaken
			}
			return models.Booking{}, err
		}
		if err = tx.Commit(); err != nil {
			return models.Booking{}, err
		}
		return booking, nil
	}
	mu.Lock()
	defer mu.Unlock()

	barber, found := findBarberByID(booking.BarberID)
	if !found {
		return models.Booking{}, ErrNotFound
	}

	if !barber.IsActive() {
		return models.Booking{}, ErrUnavailable
	}

	hairstyle, found := findHairstyleByID(booking.HairstyleID)
	if !found {
		return models.Booking{}, ErrNotFound
	}

	if hairstyle.BarberID != booking.BarberID {
		return models.Booking{}, ErrForbidden
	}

	if !hairstyle.IsAvailable {
		return models.Booking{}, ErrUnavailable
	}

	calendar, found := calendars[booking.BarberID]
	if !found {
		calendar = calendarDefault
	}
	if !calendarAllows(calendar, booking.Date, booking.Time, hairstyle.Duration, time.Now(), true) {
		return models.Booking{}, ErrOutsideSchedule
	}
	booking.BarberName = barber.FullName
	booking.ShopName = barber.ShopName
	booking.ShopAddress = barber.ShopAddress
	booking.ServiceName = hairstyle.Name
	booking.ServicePrice = hairstyle.Price
	booking.ServiceDuration = hairstyle.Duration
	booking.BookingTimezone = calendar.Timezone
	for _, existing := range bookings {
		if existing.BarberID != booking.BarberID || existing.Date != booking.Date || !existing.IsOpen() {
			continue
		}
		if existing.Time == booking.Time {
			return models.Booking{}, ErrSlotTaken
		}
		duration := existing.ServiceDuration
		if duration == 0 {
			if style, found := findHairstyleByID(existing.HairstyleID); found {
				duration = style.Duration
			}
		}
		if bookingIntervalsOverlap(booking.Time, hairstyle.Duration, existing.Time, duration) {
			return models.Booking{}, ErrSlotConflict
		}
	}

	booking.ID = nextID(&lastBookingID)

	bookings = append(bookings, booking)

	return booking, nil
}

// FindBookingByID returns a copy of the booking with the given ID.
func FindBookingByID(id int) (models.Booking, bool) {
	if databaseEnabled() {
		var v models.Booking
		err := db.QueryRow("select id,barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date::text,booking_time::text,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,coalesce(booking_timezone,'') from public.bookings where id=$1", id).Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone)
		return v, err == nil
	}
	mu.RLock()
	defer mu.RUnlock()

	for _, booking := range bookings {
		if booking.ID == id {
			return booking, true
		}
	}

	return models.Booking{}, false
}

// ListBookings returns every booking, for the admin overview.
func ListBookings() []models.Booking {
	if databaseEnabled() {
		return listBookingsDB("", 0)
	}
	mu.RLock()
	defer mu.RUnlock()

	return sortBySlot(append([]models.Booking(nil), bookings...))
}

// ListBookingsByBarber returns one barber's schedule.
func ListBookingsByBarber(barberID int) []models.Booking {
	if databaseEnabled() {
		return listBookingsDB("barber_id", barberID)
	}
	mu.RLock()
	defer mu.RUnlock()

	var owned []models.Booking

	for _, booking := range bookings {
		if booking.BarberID == barberID {
			owned = append(owned, booking)
		}
	}

	return sortBySlot(owned)
}

// ListBookingsByCustomer returns one customer's appointments.
func ListBookingsByCustomer(customerID int) []models.Booking {
	if databaseEnabled() {
		return listBookingsDB("customer_id", customerID)
	}
	mu.RLock()
	defer mu.RUnlock()

	var owned []models.Booking

	for _, booking := range bookings {
		if booking.CustomerID == customerID {
			owned = append(owned, booking)
		}
	}

	return sortBySlot(owned)
}

// TransitionBookingStatus changes a booking only when its current status is
// one of the allowed source states. The check and write happen atomically.
func TransitionBookingStatus(id int, status string, allowedFrom []string, barberID, customerID int) error {
	if len(allowedFrom) == 0 {
		return ErrInvalidTransition
	}
	if databaseEnabled() {
		q := "update public.bookings set status=$1 where id=$2"
		args := []any{status, id}
		next := 3
		if barberID != 0 {
			q += fmt.Sprintf(" and barber_id=$%d", next)
			args = append(args, barberID)
			next++
		}
		if customerID != 0 {
			q += fmt.Sprintf(" and customer_id=$%d", next)
			args = append(args, customerID)
			next++
		}
		placeholders := make([]string, len(allowedFrom))
		for i, v := range allowedFrom {
			placeholders[i] = fmt.Sprintf("$%d", next)
			args = append(args, v)
			next++
		}
		q += " and status in (" + strings.Join(placeholders, ",") + ")"
		result, err := db.Exec(q, args...)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n == 1 {
			return nil
		}
		var ownerBarber, ownerCustomer int
		var current string
		err = db.QueryRow("select barber_id,customer_id,status from public.bookings where id=$1", id).Scan(&ownerBarber, &ownerCustomer, &current)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if barberID != 0 && ownerBarber != barberID {
			return ErrForbidden
		}
		if customerID != 0 && ownerCustomer != customerID {
			return ErrForbidden
		}
		return ErrInvalidTransition
	}
	mu.Lock()
	defer mu.Unlock()
	for i := range bookings {
		if bookings[i].ID != id {
			continue
		}
		if barberID != 0 && bookings[i].BarberID != barberID {
			return ErrForbidden
		}
		if customerID != 0 && bookings[i].CustomerID != customerID {
			return ErrForbidden
		}
		allowed := false
		for _, from := range allowedFrom {
			if bookings[i].Status == from {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrInvalidTransition
		}
		bookings[i].Status = status
		return nil
	}
	return ErrNotFound
}

func bookingIntervalsOverlap(aTime string, aDuration int, bTime string, bDuration int) bool {
	parse := func(value string) (int, bool) {
		parts := strings.Split(value, ":")
		if len(parts) < 2 {
			return 0, false
		}
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0, false
		}
		return h*60 + m, true
	}
	a, okA := parse(aTime)
	b, okB := parse(bTime)
	if !okA || !okB {
		return false
	}
	return a < b+bDuration && b < a+aDuration
}

// SetBookingStatus moves a booking to a new status. The caller passes
// the ID it is allowed to act on: barberID for the barber's own
// schedule, customerID for the customer's own appointments. Passing 0
// skips that check, which only the admin does.
func SetBookingStatus(id int, status string, barberID, customerID int) error {
	if databaseEnabled() {
		q := "update public.bookings set status=$1 where id=$2"
		args := []any{status, id}
		if barberID != 0 {
			q += " and barber_id=$3"
			args = append(args, barberID)
		}
		if customerID != 0 {
			q += " and customer_id=$3"
			args = append(args, customerID)
		}
		result, err := db.Exec(q, args...)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return ErrForbidden
		}
		return nil
	}
	mu.Lock()
	defer mu.Unlock()

	for i := range bookings {
		if bookings[i].ID != id {
			continue
		}

		if barberID != 0 && bookings[i].BarberID != barberID {
			return ErrForbidden
		}

		if customerID != 0 && bookings[i].CustomerID != customerID {
			return ErrForbidden
		}

		bookings[i].Status = status

		return nil
	}

	return ErrNotFound
}

// sortBySlot orders bookings by appointment date then time, earliest
// first, so both dashboards read like a schedule.
func sortBySlot(list []models.Booking) []models.Booking {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Date != list[j].Date {
			return list[i].Date < list[j].Date
		}

		if list[i].Time != list[j].Time {
			return list[i].Time < list[j].Time
		}

		return list[i].ID < list[j].ID
	})

	return list
}

func listBookingsDB(field string, id int) []models.Booking {
	q := "select id,barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date::text,booking_time::text,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,coalesce(booking_timezone,'') from public.bookings"
	var rows *sql.Rows
	var err error
	if field != "" {
		q += " where " + field + "=$1"
		rows, err = db.Query(q, id)
	} else {
		rows, err = db.Query(q)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []models.Booking
	for rows.Next() {
		var v models.Booking
		if rows.Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone) == nil {
			out = append(out, v)
		}
	}
	return sortBySlot(out)
}
