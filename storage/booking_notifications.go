package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"sort"
)

// bookingAlertColumns mirrors the booking columns the alert poll reads. It is
// kept in one place so the database scan order cannot drift from the query.
const bookingAlertColumns = `id,barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date::text,booking_time::text,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,coalesce(booking_timezone,''),coalesce(service_image,'')`

func scanBookingAlert(rows *sql.Rows) (models.Booking, error) {
	var v models.Booking
	err := rows.Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone, &v.ServiceImage)
	return v, err
}

// PendingBookingsForBarberAfter reports the newest booking id owned by the
// barber and every still-pending booking created after afterID, oldest first.
//
// The barber's phone polls this to raise the "new booking" alert. Using the
// booking id as the cursor keeps the check cheap and monotonic: ids only grow,
// so a booking created between two polls can never be skipped.
func PendingBookingsForBarberAfter(ctx context.Context, barberID, afterID int) (int, []models.Booking, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	if barberID < 1 || afterID < 0 {
		return 0, nil, ErrInvalidPage
	}

	if !databaseEnabled() {
		latest, pending := pendingBookingsFromMemory(barberID, afterID)
		return latest, pending, nil
	}

	var latest int
	if err := db.QueryRowContext(ctx, `select coalesce(max(id),0) from public.bookings where barber_id=$1`, barberID).Scan(&latest); err != nil {
		return 0, nil, err
	}

	pending, err := queryRecords(ctx, `select `+bookingAlertColumns+` from public.bookings where barber_id=$1 and status='Pending' and id>$2 order by id asc limit 25`, scanBookingAlert, barberID, afterID)
	if err != nil {
		return 0, nil, err
	}
	return latest, pending, nil
}

// pendingBookingsFromMemory is the in-memory half of the read. It holds the
// store lock once so it cannot deadlock with a nested ListBookingsByBarber call.
func pendingBookingsFromMemory(barberID, afterID int) (int, []models.Booking) {
	mu.RLock()
	defer mu.RUnlock()

	latest := 0
	pending := make([]models.Booking, 0)
	for _, booking := range bookings {
		if booking.BarberID != barberID {
			continue
		}
		if booking.ID > latest {
			latest = booking.ID
		}
		if booking.Status == models.StatusPending && booking.ID > afterID {
			pending = append(pending, booking)
		}
	}

	sort.Slice(pending, func(i, j int) bool { return pending[i].ID < pending[j].ID })
	if len(pending) > 25 {
		pending = pending[len(pending)-25:]
	}
	return latest, pending
}