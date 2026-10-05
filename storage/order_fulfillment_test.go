package storage

import (
	"barber-booking/models"
	"context"
	"errors"
	"testing"
)

func TestOrderFulfillment(t *testing.T) { Reset(); t.Cleanup(Reset); checkOrderFulfillment(t) }
func checkOrderFulfillment(t *testing.T) {
	ctx := context.Background()
	t.Run("multi_seller_state_and_stock", func(t *testing.T) {
		first, buyer, _ := postgresBookingFixture(t)
		second := newBarber(t, "second_"+first.Email)
		a := postgresProduct(t, first.ID, 3)
		b := postgresProduct(t, second.ID, 4)
		for _, p := range []models.Product{a, b} {
			if _, err := AddToCart(buyer.ID, p.ID); err != nil {
				t.Fatal(err)
			}
		}
		order, err := CheckoutCart(buyer)
		if err != nil {
			t.Fatal(err)
		}
		items, err := ListProductOrderItems(ctx, order.ID)
		if err != nil || len(items) != 2 {
			t.Fatalf("items: %+v %v", items, err)
		}
		var ai, bi models.ProductOrderItem
		for _, item := range items {
			if item.BarberID == first.ID {
				ai = item
			} else {
				bi = item
			}
			if item.FulfillmentStatus != models.FulfillmentPending {
				t.Fatalf("initial item state: %+v", item)
			}
		}
		checkStatus := func(want string) {
			t.Helper()
			stored, err := FindProductOrderByIDResult(ctx, order.ID)
			if err != nil || stored.Status != want {
				t.Fatalf("order status: %+v %v, want %s", stored, err, want)
			}
		}
		for _, invalid := range []struct {
			owner int
			state string
		}{{second.ID, models.FulfillmentReady}, {0, models.FulfillmentReady}, {first.ID, "invented"}, {first.ID, models.FulfillmentCollected}} {
			if err := FulfillOrderItem(ctx, ai.ID, invalid.owner, invalid.state); err == nil {
				t.Fatalf("invalid fulfillment succeeded: %+v", invalid)
			}
		}
		if err := FulfillOrderItem(ctx, ai.ID, first.ID, models.FulfillmentReady); err != nil {
			t.Fatal(err)
		}
		checkStatus(models.OrderPending)
		if err := FulfillOrderItem(ctx, bi.ID, second.ID, models.FulfillmentReady); err != nil {
			t.Fatal(err)
		}
		checkStatus(models.OrderReady)
		// Different sellers act concurrently; aggregate order state must see both.
		outcomes := concurrentErrors(func() error { return FulfillOrderItem(ctx, ai.ID, first.ID, models.FulfillmentCollected) }, func() error { return FulfillOrderItem(ctx, bi.ID, second.ID, models.FulfillmentCancelled) })
		for _, err := range outcomes {
			if err != nil {
				t.Fatal(err)
			}
		}
		checkStatus(models.OrderCompleted)
		pa, err := FindProductByIDResult(ctx, a.ID)
		if err != nil || pa.Quantity != 2 {
			t.Fatalf("collected stock: %+v %v", pa, err)
		}
		pb, err := FindProductByIDResult(ctx, b.ID)
		if err != nil || pb.Quantity != 4 {
			t.Fatalf("cancelled stock: %+v %v", pb, err)
		}
		for _, item := range []models.ProductOrderItem{ai, bi} {
			for _, target := range []string{models.FulfillmentReady, models.FulfillmentCollected, models.FulfillmentCancelled} {
				if err := FulfillOrderItem(ctx, item.ID, item.BarberID, target); !errors.Is(err, ErrInvalidTransition) {
					t.Fatalf("reopened closed item: %v", err)
				}
			}
		}
		pb, _ = FindProductByIDResult(ctx, b.ID)
		if pb.Quantity != 4 {
			t.Fatal("duplicate cancellation restored stock twice")
		}
		sellers, err := ListProductOrderItemsByBarber(ctx, second.ID)
		if err != nil || len(sellers) != 1 || sellers[0].FulfillmentStatus != models.FulfillmentCancelled {
			t.Fatalf("seller status: %+v %v", sellers, err)
		}
	})
	t.Run("duplicate_cancellation_race", func(t *testing.T) {
		seller, buyer, _ := postgresBookingFixture(t)
		p := postgresProduct(t, seller.ID, 2)
		if _, err := AddToCart(buyer.ID, p.ID); err != nil {
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
		outcomes := concurrentErrors(func() error { return FulfillOrderItem(ctx, items[0].ID, seller.ID, models.FulfillmentCancelled) }, func() error { return FulfillOrderItem(ctx, items[0].ID, seller.ID, models.FulfillmentCancelled) })
		assertOneSuccess(t, outcomes, ErrInvalidTransition)
		product, err := FindProductByIDResult(ctx, p.ID)
		if err != nil || product.Quantity != 2 {
			t.Fatalf("stock: %+v %v", product, err)
		}
		stored, err := FindProductOrderByIDResult(ctx, order.ID)
		if err != nil || stored.Status != models.OrderCancelled {
			t.Fatalf("cancelled order: %+v %v", stored, err)
		}
	})
}
