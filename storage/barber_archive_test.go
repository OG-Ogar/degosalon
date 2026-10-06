package storage

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	"barber-booking/models"
	"barber-booking/session"
)

func TestBarberArchive(t *testing.T) { Reset(); t.Cleanup(Reset); checkBarberArchive(t) }

func checkBarberArchive(t *testing.T) {
	ctx := context.Background()
	seller, buyer, style := postgresBookingFixture(t)
	booking, err := CreateBooking(newBooking(buyer, seller.ID, style.ID))
	if err != nil {
		t.Fatal(err)
	}
	if err := ArchiveBarber(ctx, seller.ID); !errors.Is(err, ErrOutstandingWork) {
		t.Fatalf("open appointment: %v", err)
	}
	if err := SetBookingStatus(booking.ID, models.StatusAccepted, seller.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := ArchiveBarber(ctx, seller.ID); !errors.Is(err, ErrOutstandingWork) {
		t.Fatalf("accepted appointment: %v", err)
	}
	if err := SetBookingStatus(booking.ID, models.StatusCompleted, seller.ID, 0); err != nil {
		t.Fatal(err)
	}
	product := postgresProduct(t, seller.ID, 3)
	if _, err := AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	order, err := CheckoutCart(buyer)
	if err != nil {
		t.Fatal(err)
	}
	items, err := ListProductOrderItems(ctx, order.ID)
	if err != nil || len(items) != 1 {
		t.Fatal(err)
	}
	if err := ArchiveBarber(ctx, seller.ID); !errors.Is(err, ErrOutstandingWork) {
		t.Fatalf("uncollected order: %v", err)
	}
	if err := FulfillOrderItem(ctx, items[0].ID, seller.ID, models.FulfillmentReady); err != nil {
		t.Fatal(err)
	}
	if err := ArchiveBarber(ctx, seller.ID); !errors.Is(err, ErrOutstandingWork) {
		t.Fatalf("ready order: %v", err)
	}
	if err := FulfillOrderItem(ctx, items[0].ID, seller.ID, models.FulfillmentCollected); err != nil {
		t.Fatal(err)
	}
	message, err := AddMessage(ctx, models.Message{BarberID: seller.ID, CustomerID: buyer.ID, SenderRole: "customer", Text: "Retain this conversation"})
	if err != nil {
		t.Fatal(err)
	}
	var memorySession session.Session
	if databaseEnabled() {
		_, err = db.ExecContext(ctx, "insert into degosalon_internal.auth_sessions(id_digest,role,user_id,csrf_token,expires_at) values(repeat('a',64),'barber',$1,'token',now()+interval '1 hour')", seller.ID)
	} else {
		memorySession, err = session.New(session.RoleBarber, seller.ID)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := ArchiveBarber(ctx, seller.ID); err != nil {
		t.Fatal(err)
	}
	if err := ArchiveBarber(ctx, seller.ID); err != nil {
		t.Fatalf("retry: %v", err)
	}
	retained, err := FindBarberByIDContext(ctx, seller.ID)
	if err != nil || retained.Status != models.BarberArchived || retained.ShopName != seller.ShopName {
		t.Fatalf("retained account: %+v %v", retained, err)
	}
	if err := SetBarberStatusContext(ctx, seller.ID, models.BarberActive); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reactivation: %v", err)
	}
	if err := UpdateBarberProfile(ctx, seller); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale profile: %v", err)
	}
	if err := UpdateBarber(seller); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale legacy write: %v", err)
	}
	if databaseEnabled() {
		var count int
		if err := db.QueryRowContext(ctx, "select count(*) from degosalon_internal.auth_sessions where role='barber' and user_id=$1", seller.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("retained sessions: %d %v", count, err)
		}
		if _, err := db.ExecContext(ctx, "update public.barbers set status='Active' where id=$1", seller.ID); err == nil {
			t.Fatal("SQL reopened archive")
		}
	} else if _, err := session.GetContext(ctx, memorySession.ID); err == nil {
		t.Fatal("retained memory session")
	}
	if stored, err := FindBookingByIDResult(ctx, booking.ID); err != nil || stored.ServiceName != booking.ServiceName || stored.Status != models.StatusCompleted {
		t.Fatalf("history lost: %+v %v", stored, err)
	}
	if stored, err := ListProductOrderItems(ctx, order.ID); err != nil || len(stored) != 1 || stored[0].ProductName != items[0].ProductName {
		t.Fatalf("order history lost: %+v %v", stored, err)
	}
	if stored, err := ListConversation(ctx, seller.ID, buyer.ID); err != nil || len(stored) != 1 || stored[0].ID != message.ID {
		t.Fatalf("conversation lost: %+v %v", stored, err)
	}
	if _, err := CreateBooking(newBooking(buyer, seller.ID, style.ID)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("archived booking: %v", err)
	}
	if _, err := AddToCart(buyer.ID, product.ID); err == nil {
		t.Fatal("archived seller cart")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := ArchiveBarber(canceled, seller.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
	if err := ArchiveBarber(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestBarberArchiveBookingRace(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	checkBarberArchiveBookingRace(t)
}
func checkBarberArchiveBookingRace(t *testing.T) {
	for i := 0; i < 5; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			seller, buyer, style := postgresBookingFixture(t)
			start := make(chan struct{})
			var archiveErr, bookingErr error
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); <-start; archiveErr = ArchiveBarber(context.Background(), seller.ID) }()
			go func() {
				defer wg.Done()
				<-start
				_, bookingErr = CreateBooking(newBooking(buyer, seller.ID, style.ID))
			}()
			close(start)
			wg.Wait()
			if archiveErr == nil {
				if !errors.Is(bookingErr, ErrUnavailable) {
					t.Fatalf("archive winner allowed booking: %v", bookingErr)
				}
			} else if !errors.Is(archiveErr, ErrOutstandingWork) || bookingErr != nil {
				t.Fatalf("booking winner: archive=%v booking=%v", archiveErr, bookingErr)
			}
		})
	}
}
