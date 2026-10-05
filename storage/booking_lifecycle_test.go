package storage

import (
	"barber-booking/models"
	"context"
	"errors"
	"testing"
)

func TestBookingLifecycle(t *testing.T) { Reset(); t.Cleanup(Reset); checkBookingLifecycle(t) }

func checkBookingLifecycle(t *testing.T) {
	states := []string{models.StatusPending, models.StatusAccepted, models.StatusRejected, models.StatusCancelled, models.StatusCompleted, "invented"}
	for _, source := range states[:5] {
		for _, target := range states {
			for _, actor := range []string{"barber", "customer"} {
				t.Run(source+"_"+target+"_"+actor, func(t *testing.T) {
					barber, customer, style := postgresBookingFixture(t)
					booking, err := CreateBooking(newBooking(customer, barber.ID, style.ID))
					if err != nil {
						t.Fatal(err)
					}
					if source == models.StatusAccepted || source == models.StatusCompleted {
						if err := SetBookingStatus(booking.ID, models.StatusAccepted, barber.ID, 0); err != nil {
							t.Fatal(err)
						}
					}
					switch source {
					case models.StatusCompleted, models.StatusRejected:
						err = SetBookingStatus(booking.ID, source, barber.ID, 0)
					case models.StatusCancelled:
						err = SetBookingStatus(booking.ID, source, 0, customer.ID)
					}
					if err != nil {
						t.Fatal(err)
					}
					bid, cid := barber.ID, 0
					if actor == "customer" {
						bid, cid = 0, customer.ID
					}
					allowed := (actor == "customer" && target == models.StatusCancelled && (source == models.StatusPending || source == models.StatusAccepted)) || (actor == "barber" && ((source == models.StatusPending && (target == models.StatusAccepted || target == models.StatusRejected)) || (source == models.StatusAccepted && target == models.StatusCompleted)))
					// Deliberately supply every source state: callers cannot widen the domain graph.
					err = TransitionBookingStatusContext(context.Background(), booking.ID, target, states, bid, cid)
					if allowed && err != nil {
						t.Fatalf("valid transition: %v", err)
					}
					if !allowed && !errors.Is(err, ErrInvalidTransition) {
						t.Fatalf("invalid transition: %v", err)
					}
					stored, err := FindBookingByIDResult(context.Background(), booking.ID)
					if err != nil {
						t.Fatal(err)
					}
					want := source
					if allowed {
						want = target
					}
					if stored.Status != want {
						t.Fatalf("status %s, want %s", stored.Status, want)
					}
				})
			}
		}
	}
	t.Run("identity_creation_and_legacy_guards", func(t *testing.T) {
		barber, customer, style := postgresBookingFixture(t)
		request := newBooking(customer, barber.ID, style.ID)
		for _, status := range states[1:] {
			request.Status = status
			if _, err := CreateBooking(request); !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("created %s: %v", status, err)
			}
		}
		request.Status = ""
		booking, err := CreateBooking(request)
		if err != nil || booking.Status != models.StatusPending {
			t.Fatalf("default pending: %+v %v", booking, err)
		}
		for _, owner := range [][2]int{{0, 0}, {barber.ID, customer.ID}, {-1, 0}, {0, -1}, {barber.ID + 1000000, 0}, {0, customer.ID + 1000000}} {
			if err := SetBookingStatus(booking.ID, models.StatusCancelled, owner[0], owner[1]); err == nil {
				t.Fatalf("owner bypass: %v", owner)
			}
		}
		if err := SetBookingStatus(booking.ID, models.StatusCompleted, barber.ID, 0); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("legacy bypass: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := TransitionBookingStatusContext(ctx, booking.ID, models.StatusAccepted, []string{models.StatusPending}, barber.ID, 0); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled context: %v", err)
		}
	})
}
