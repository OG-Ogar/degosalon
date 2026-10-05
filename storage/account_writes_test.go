package storage

import (
	"context"
	"errors"
	"testing"

	"barber-booking/models"
)

func TestProfileEditsPreserveConcurrentRecoveryAndSuspension(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	checkProfileEditsPreserveRecovery(t)
}

func checkProfileEditsPreserveRecovery(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	customer := newCustomer(t, "profile_customer@example.com")
	barber := newBarber(t, "profile_barber@example.com")
	// These copies predate the reset and suspension writes.
	customer.PasswordHash = "stale customer password"
	barber.PasswordHash = "stale barber password"
	customer.FullName = "Updated customer"
	barber.ShopName = "Updated shop"
	if err := SetCustomerPasswordContext(ctx, customer.ID, "new customer password"); err != nil {
		t.Fatal(err)
	}
	if err := SetBarberPasswordContext(ctx, barber.ID, "new barber password"); err != nil {
		t.Fatal(err)
	}
	if err := SetBarberStatusContext(ctx, barber.ID, models.BarberSuspended); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCustomerProfile(ctx, customer); err != nil {
		t.Fatal(err)
	}
	if err := UpdateBarberProfile(ctx, barber); err != nil {
		t.Fatal(err)
	}
	gotCustomer, err := FindCustomerByIDResult(ctx, customer.ID)
	if err != nil || gotCustomer.PasswordHash != "new customer password" || gotCustomer.FullName != customer.FullName {
		t.Fatalf("customer profile: %+v %v", gotCustomer, err)
	}
	gotBarber, err := FindBarberByIDContext(ctx, barber.ID)
	if err != nil || gotBarber.PasswordHash != "new barber password" || gotBarber.Status != models.BarberSuspended || gotBarber.ShopName != barber.ShopName {
		t.Fatalf("barber profile: %+v %v", gotBarber, err)
	}
	otherCustomer := newCustomer(t, "other_profile_customer@example.com")
	otherBarber := newBarber(t, "other_profile_barber@example.com")
	customer.Email = otherCustomer.Email
	barber.Email = otherBarber.Email
	if err := UpdateCustomerProfile(ctx, customer); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("customer duplicate: %v", err)
	}
	if err := UpdateBarberProfile(ctx, barber); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("barber duplicate: %v", err)
	}
	for name, write := range accountWriteCalls() {
		if err := write(ctx); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s missing: %v", name, err)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if err := write(canceled); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s cancellation: %v", name, err)
		}
	}
}

func accountWriteCalls() map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"customer password": func(ctx context.Context) error { return SetCustomerPasswordContext(ctx, 99999, "hash") },
		"barber password":   func(ctx context.Context) error { return SetBarberPasswordContext(ctx, 99999, "hash") },
		"customer profile": func(ctx context.Context) error {
			return UpdateCustomerProfile(ctx, models.Customer{ID: 99999, Email: "missing@example.com"})
		},
		"barber profile": func(ctx context.Context) error {
			return UpdateBarberProfile(ctx, models.Barber{ID: 99999, Email: "missing@example.com"})
		},
		"barber status": func(ctx context.Context) error { return SetBarberStatusContext(ctx, 99999, models.BarberSuspended) },
	}
}
