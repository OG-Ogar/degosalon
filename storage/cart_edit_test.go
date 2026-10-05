package storage

import (
	"context"
	"errors"
	"testing"

	"barber-booking/models"
)

func TestCartEditing(t *testing.T) { Reset(); t.Cleanup(Reset); checkCartEditing(t) }

func checkCartEditing(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	barber := newBarber(t, "cart_edit_barber@example.com")
	customer := newCustomer(t, "cart_edit_customer@example.com")
	other := newCustomer(t, "cart_edit_other@example.com")
	product := postgresProduct(t, barber.ID, 3)
	empty := postgresProduct(t, barber.ID, 0)
	if _, err := AddToCartContext(ctx, customer.ID, empty.ID); !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("zero stock add: %v", err)
	}
	if _, err := AddToCartContext(ctx, customer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	if err := SetCartQuantity(ctx, other.ID, product.ID, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unowned remove: %v", err)
	}
	if err := SetCartQuantity(ctx, customer.ID, product.ID, 2); err != nil {
		t.Fatal(err)
	}
	if count, err := CartItemCount(ctx, customer.ID); err != nil || count != 2 {
		t.Fatalf("quantity: %d %v", count, err)
	}
	if err := SetCartQuantity(ctx, customer.ID, product.ID, 4); !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("oversized quantity: %v", err)
	}
	if err := SetCartQuantity(ctx, customer.ID, product.ID, -1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("negative quantity: %v", err)
	}
	stored, err := FindProductByIDResult(ctx, product.ID)
	if err != nil || stored.Quantity != 3 {
		t.Fatalf("cart edit reserved stock: %+v %v", stored, err)
	}
	if err := SetBarberStatusContext(ctx, barber.ID, models.BarberSuspended); err != nil {
		t.Fatal(err)
	}
	if err := SetCartQuantity(ctx, customer.ID, product.ID, 1); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("suspended seller update: %v", err)
	}
	if err := SetCartQuantity(ctx, customer.ID, product.ID, 0); err != nil {
		t.Fatalf("unavailable removal: %v", err)
	}
	if count, err := CartItemCount(ctx, customer.ID); err != nil || count != 0 {
		t.Fatalf("remove count: %d %v", count, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := AddToCartContext(canceled, customer.ID, product.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("add cancellation: %v", err)
	}
	if err := SetCartQuantity(canceled, customer.ID, product.ID, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("edit cancellation: %v", err)
	}
	if _, err := CheckoutCartContext(canceled, customer); !errors.Is(err, context.Canceled) {
		t.Fatalf("checkout cancellation: %v", err)
	}
}
