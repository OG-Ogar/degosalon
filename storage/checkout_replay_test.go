package storage

import (
	"context"
	"errors"
	"testing"

	"barber-booking/models"
)

func TestCheckoutReplay(t *testing.T) { Reset(); t.Cleanup(Reset); checkCheckoutReplay(t) }

func checkCheckoutReplay(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	barber := newBarber(t, "checkout_replay_barber@example.com")
	customer := newCustomer(t, "checkout_replay_customer@example.com")
	other := newCustomer(t, "checkout_replay_other@example.com")
	product := postgresProduct(t, barber.ID, 4)
	if _, err := AddToCart(customer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	key := "0123456789abcdef0123456789abcdef"
	for _, invalid := range []string{"", "short", "0123456789ABCDEF0123456789ABCDEF", "0123456789abcdef0123456789abcdeg"} {
		if _, err := CheckoutCartWithKey(ctx, customer, invalid); !errors.Is(err, ErrInvalidCheckoutKey) {
			t.Fatalf("invalid key accepted: %v", err)
		}
	}
	results := make(chan models.ProductOrder, 2)
	outcomes := concurrentErrors(func() error { order, err := CheckoutCartWithKey(ctx, customer, key); results <- order; return err }, func() error { order, err := CheckoutCartWithKey(ctx, customer, key); results <- order; return err })
	for _, err := range outcomes {
		if err != nil {
			t.Fatal(err)
		}
	}
	first, second := <-results, <-results
	if first.ID == 0 || first.ID != second.ID {
		t.Fatalf("different replay orders: %+v %+v", first, second)
	}
	if _, err := AddToCart(customer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	replay, err := CheckoutCartWithKey(ctx, customer, key)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	count, err := CartItemCount(ctx, customer.ID)
	if err != nil || count != 1 {
		t.Fatalf("replay consumed new cart: %d %v", count, err)
	}
	stored, err := FindProductByIDResult(ctx, product.ID)
	if err != nil || stored.Quantity != 3 {
		t.Fatalf("replay decremented stock: %+v %v", stored, err)
	}
	orders, err := ListProductOrdersByCustomer(ctx, customer.ID)
	if err != nil || len(orders) != 1 {
		t.Fatalf("duplicate orders: %+v %v", orders, err)
	}
	// Another account's identical key must never reveal or replay this order.
	if _, err := CheckoutCartWithKey(ctx, other, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-customer replay: %v", err)
	}
	if _, err := AddToCart(other.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	otherOrder, err := CheckoutCartWithKey(ctx, other, key)
	if err != nil || otherOrder.ID == first.ID || otherOrder.CustomerID != other.ID {
		t.Fatalf("scoped key: %+v %v", otherOrder, err)
	}
	// Failed validation does not claim a key; retry succeeds after stock returns.
	retryKey := "fedcba9876543210fedcba9876543210"
	stored.Quantity = 0
	if err := UpdateProduct(stored); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckoutCartWithKey(ctx, customer, retryKey); err == nil {
		t.Fatal("checkout succeeded without stock")
	}
	stored.Quantity = 2
	if err := UpdateProduct(stored); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckoutCartWithKey(ctx, customer, retryKey); err != nil {
		t.Fatalf("failed request claimed retry key: %v", err)
	}
}
