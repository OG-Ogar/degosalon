package storage

import (
	"barber-booking/models"
	"context"
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
	if v.Rating < 1 || v.Rating > 5 {
		return models.Review{}, ErrForbidden
	}
	v.Comment = strings.TrimSpace(v.Comment)
	if len(v.Comment) > 2000 {
		v.Comment = v.Comment[:2000]
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	booking, err := CanCustomerReviewResult(ctx, v.CustomerID, v.BarberID)
	if errors.Is(err, ErrNotFound) {
		return models.Review{}, ErrForbidden
	}
	if err != nil {
		return models.Review{}, err
	}
	v.BookingID = booking.ID
	if databaseEnabled() {
		err := db.QueryRowContext(ctx, "insert into public.reviews(barber_id,customer_id,booking_id,customer_name,rating,comment,created_at) values($1,$2,$3,$4,$5,$6,$7) returning id", v.BarberID, v.CustomerID, v.BookingID, v.CustomerName, v.Rating, v.Comment, v.CreatedAt).Scan(&v.ID)
		return v, err
	}
	mu.Lock()
	defer mu.Unlock()
	lastReviewID++
	v.ID = lastReviewID
	reviews = append(reviews, v)
	return v, nil
}
