package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBookingSnapshots(t *testing.T) { Reset(); t.Cleanup(Reset); checkBookingSnapshots(t) }

func checkBookingSnapshots(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	barber := newBarber(t, "snapshot_barber@example.com")
	customer := newCustomer(t, "snapshot_customer@example.com")
	style := newHairstyle(t, barber.ID)
	style.Duration = 60
	style.Image = "/uploads/original-style.jpg"
	if err := UpdateHairstyle(style); err != nil {
		t.Fatal(err)
	}
	request := newBooking(customer, barber.ID, style.ID)
	request.ServiceName = "untrusted name"
	request.ServicePrice = 0.01
	request.ServiceDuration = 1
	booking, err := CreateBookingContext(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if booking.ServiceName != style.Name || booking.ServiceDuration != 60 || booking.ServicePrice != style.Price || booking.ServiceImage != style.Image || booking.ShopName != barber.ShopName {
		t.Fatalf("snapshot not server-owned: %+v", booking)
	}
	style.Name = "Edited service"
	style.Duration = 15
	style.Price = 99999
	style.Image = "/uploads/edited-style.jpg"
	if err := UpdateHairstyle(style); err != nil {
		t.Fatal(err)
	}
	barber.ShopName = "Edited shop"
	barber.FullName = "Edited barber"
	if err := UpdateBarberProfile(ctx, barber); err != nil {
		t.Fatal(err)
	}
	stored, err := FindBookingByIDResult(ctx, booking.ID)
	if err != nil || stored.ServiceName != booking.ServiceName || stored.ServiceDuration != 60 || stored.ServicePrice != booking.ServicePrice || stored.ServiceImage != "/uploads/original-style.jpg" || stored.ShopName != booking.ShopName || stored.BarberName != booking.BarberName {
		t.Fatalf("history changed: %+v %v", stored, err)
	}
	page, err := BookingPage(ctx, BookingFilter{CustomerID: customer.ID, Group: "all", Now: time.Now()}, PageCursor{})
	if err != nil || len(page.Items) != 1 || page.Items[0].HairstyleImage != "/uploads/original-style.jpg" {
		t.Fatalf("booking page photo snapshot: %+v %v", page.Items, err)
	}
	overlapping := request
	overlapping.Time = "10:30"
	if _, err := CreateBookingContext(ctx, overlapping); !errors.Is(err, ErrSlotConflict) {
		t.Fatalf("edited duration shortened existing appointment: %v", err)
	}
	adjacent := request
	adjacent.Time = "11:00"
	next, err := CreateBookingContext(ctx, adjacent)
	if err != nil || next.ServiceDuration != 15 || next.ServiceName != style.Name || next.ServicePrice != style.Price {
		t.Fatalf("new booking snapshot: %+v %v", next, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := CreateBookingContext(canceled, adjacent); !errors.Is(err, context.Canceled) {
		t.Fatalf("creation cancellation: %v", err)
	}
	if err := SaveBookingCustomerPhoneContext(ctx, booking.ID, customer.ID, "08012345678"); err != nil {
		t.Fatalf("save confirmation phone: %v", err)
	}
	stored, err = FindBookingByIDResult(ctx, booking.ID)
	updatedCustomer, customerErr := FindCustomerByIDResult(ctx, customer.ID)
	if err != nil || customerErr != nil || stored.CustomerPhone != "08012345678" || updatedCustomer.PhoneNumber != "08012345678" {
		t.Fatalf("phone not saved to customer and booking: booking=%+v customer=%+v errors=%v/%v", stored, updatedCustomer, err, customerErr)
	}
	if err := SaveBookingCustomerPhoneContext(ctx, booking.ID, customer.ID+1, "08012345678"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("phone update accepted for non-owner: %v", err)
	}
}
