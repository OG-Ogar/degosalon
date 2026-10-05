package storage

import (
	"barber-booking/models"
	"context"
	"testing"
)

func TestCompletedBookingCanCreateOneReview(t *testing.T) {
	Reset()
	barbers = []models.Barber{{ID: 1, FullName: "B", ShopName: "Shop", State: "Lagos", LocalArea: "Ikeja", Status: models.BarberActive}}
	customers = []models.Customer{{ID: 1, FullName: "Customer"}}
	bookings = []models.Booking{{ID: 1, BarberID: 1, CustomerID: 1, Status: models.StatusCompleted}}
	got, err := CreateReview(context.Background(), models.Review{BarberID: 1, CustomerID: 1, CustomerName: "Customer", Rating: 5, Comment: "Great cut"})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if got.BookingID != 1 {
		t.Fatalf("booking id=%d want 1", got.BookingID)
	}
	if _, err := CreateReview(context.Background(), models.Review{BarberID: 1, CustomerID: 1, CustomerName: "Customer", Rating: 4}); err == nil {
		t.Fatal("second review for same booking should fail")
	}
	avg, count := ReviewSummary(1)
	if avg != 5 || count != 1 {
		t.Fatalf("summary=(%v,%d), want (5,1)", avg, count)
	}
}

func TestPendingBookingCannotReview(t *testing.T) {
	Reset()
	bookings = []models.Booking{{ID: 1, BarberID: 1, CustomerID: 1, Status: models.StatusPending}}
	if _, err := CreateReview(context.Background(), models.Review{BarberID: 1, CustomerID: 1, Rating: 5}); err == nil {
		t.Fatal("pending booking must not permit review")
	}
}
