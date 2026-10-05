package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"barber-booking/models"
)

func TestReviewIntegrity(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	checkReviewIntegrity(t)
}

func checkReviewIntegrity(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	barber := newBarber(t, "review_integrity_barber@example.com")
	customer := newCustomer(t, "review_integrity_customer@example.com")
	other := newCustomer(t, "review_integrity_other@example.com")
	style := newHairstyle(t, barber.ID)
	booking, err := CreateBooking(newBooking(customer, barber.ID, style.ID))
	if err != nil {
		t.Fatal(err)
	}
	candidate := models.Review{CustomerID: customer.ID, BarberID: barber.ID, Rating: 5, CustomerName: "forged name", Comment: strings.Repeat("好", 2001)}
	if _, err := CreateReview(ctx, candidate); !errors.Is(err, ErrForbidden) {
		t.Fatalf("pending review: %v", err)
	}
	if err := TransitionBookingStatus(booking.ID, models.StatusAccepted, []string{models.StatusPending}, barber.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := TransitionBookingStatus(booking.ID, models.StatusCompleted, []string{models.StatusAccepted}, barber.ID, 0); err != nil {
		t.Fatal(err)
	}
	forged := candidate
	forged.CustomerID = other.ID
	if _, err := CreateReview(ctx, forged); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unowned booking: %v", err)
	}
	outcomes := concurrentErrors(func() error { _, err := CreateReview(ctx, candidate); return err }, func() error { _, err := CreateReview(ctx, candidate); return err })
	assertOneSuccess(t, outcomes, ErrForbidden)
	list, err := ListReviewsByBarberResult(ctx, barber.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("reviews: %+v %v", list, err)
	}
	review := list[0]
	if review.BookingID != booking.ID || review.CustomerName != customer.FullName || !utf8.ValidString(review.Comment) || utf8.RuneCountInString(review.Comment) != 2000 {
		t.Fatalf("review snapshot invalid: %+v", review)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := CreateReview(canceled, candidate); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled review: %v", err)
	}
}
