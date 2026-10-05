package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var reviews []models.Review
var lastReviewID int

func ListReviewsByBarber(barberID int) []models.Review {
	if databaseEnabled() {
		rows, err := db.Query("select id,barber_id,customer_id,booking_id,customer_name,rating,comment,created_at from public.reviews where barber_id=$1 order by created_at desc", barberID)
		if err == nil {
			defer rows.Close()
			var out []models.Review
			for rows.Next() {
				var v models.Review
				if rows.Scan(&v.ID, &v.BarberID, &v.CustomerID, &v.BookingID, &v.CustomerName, &v.Rating, &v.Comment, &v.CreatedAt) == nil {
					out = append(out, v)
				}
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	var out []models.Review
	for _, v := range reviews {
		if v.BarberID == barberID {
			out = append(out, v)
		}
	}
	return out
}

func ReviewSummary(barberID int) (float64, int) {
	list := ListReviewsByBarber(barberID)
	if len(list) == 0 {
		return 0, 0
	}
	total := 0
	for _, v := range list {
		total += v.Rating
	}
	return float64(total) / float64(len(list)), len(list)
}

func CanCustomerReview(customerID, barberID int) (models.Booking, bool) {
	for _, b := range ListBookingsByCustomer(customerID) {
		if b.BarberID == barberID && b.Status == models.StatusCompleted {
			if databaseEnabled() {
				var n int
				_ = db.QueryRow("select count(*) from public.reviews where booking_id=$1", b.ID).Scan(&n)
				if n == 0 {
					return b, true
				}
			} else {
				used := false
				mu.RLock()
				for _, r := range reviews {
					if r.BookingID == b.ID {
						used = true
						break
					}
				}
				mu.RUnlock()
				if !used {
					return b, true
				}
			}
		}
	}
	return models.Booking{}, false
}

func CreateReview(ctx context.Context, v models.Review) (models.Review, error) {
	if err := ctx.Err(); err != nil {
		return models.Review{}, err
	}
	if v.Rating < 1 || v.Rating > 5 {
		return models.Review{}, ErrForbidden
	}
	comment := []rune(strings.TrimSpace(v.Comment))
	if len(comment) > 2000 {
		comment = comment[:2000]
	}
	v.Comment = string(comment)
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	if databaseEnabled() {
		// Eligibility and insertion share one statement. The unique booking key
		// arbitrates concurrent submissions without exposing a database exception.
		err := db.QueryRowContext(ctx, `insert into public.reviews(barber_id,customer_id,booking_id,customer_name,rating,comment,created_at)
   select b.barber_id,b.customer_id,b.id,c.full_name,$3,$4,$5
   from public.bookings b join public.customers c on c.id=b.customer_id
   where b.customer_id=$1 and b.barber_id=$2 and b.status='Completed'
   and not exists(select 1 from public.reviews r where r.booking_id=b.id)
   order by b.id limit 1
   on conflict(booking_id) do nothing returning id,booking_id,customer_name`, v.CustomerID, v.BarberID, v.Rating, v.Comment, v.CreatedAt).Scan(&v.ID, &v.BookingID, &v.CustomerName)
		if errors.Is(err, sql.ErrNoRows) {
			return models.Review{}, ErrForbidden
		}
		if err != nil {
			return models.Review{}, err
		}
		return v, nil
	}
	mu.Lock()
	defer mu.Unlock()
	// Keep the eligibility check inside the insertion lock.
	for _, booking := range bookings {
		if booking.CustomerID != v.CustomerID || booking.BarberID != v.BarberID || booking.Status != models.StatusCompleted {
			continue
		}
		used := false
		for _, review := range reviews {
			if review.BookingID == booking.ID {
				used = true
				break
			}
		}
		if used {
			continue
		}
		v.BookingID = booking.ID
		if customer, found := findCustomerByID(v.CustomerID); found {
			v.CustomerName = customer.FullName
		}
		lastReviewID++
		v.ID = lastReviewID
		reviews = append(reviews, v)
		return v, nil
	}
	return models.Review{}, ErrForbidden
}
