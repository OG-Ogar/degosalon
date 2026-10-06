package storage

import (
	"context"
	"errors"
	"testing"

	"barber-booking/models"
)

func TestCatalogueArchives(t *testing.T) { Reset(); t.Cleanup(Reset); checkCatalogueArchives(t) }

func checkCatalogueArchives(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	seller, buyer, style := postgresBookingFixture(t)
	uri := mediaURI(9001)
	if err := TrackMediaContext(ctx, uri); err != nil {
		t.Fatal(err)
	}
	style.Image = uri
	style.Video = "/uploads/hairstyle-videos/1234567890123.mp4"
	if err := UpdateHairstyle(style); err != nil {
		t.Fatal(err)
	}
	booking, err := CreateBooking(newBooking(buyer, seller.ID, style.ID))
	if err != nil {
		t.Fatal(err)
	}
	product := postgresProduct(t, seller.ID, 3)
	product.Image = uri
	if err := UpdateProduct(product); err != nil {
		t.Fatal(err)
	}
	if _, err := AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	order, err := CheckoutCart(buyer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	items, err := ListProductOrderItems(ctx, order.ID)
	if err != nil || len(items) != 1 {
		t.Fatal(err)
	}
	// Neither owner-scoped operation can archive a different seller's entry.
	if _, err := DeleteProduct(product.ID, seller.ID+99999); err == nil {
		t.Fatal("cross seller product archive")
	}
	if _, err := DeleteHairstyle(style.ID, seller.ID+99999); err == nil {
		t.Fatal("cross seller style archive")
	}
	if path, err := DeleteProduct(product.ID, seller.ID); err != nil || path != "" {
		t.Fatal("product archive requested media deletion", err)
	}
	if path, err := DeleteHairstyle(style.ID, seller.ID); err != nil || path != "" {
		t.Fatal("style archive requested media deletion", err)
	}
	retainedProduct, err := FindProductByIDResult(ctx, product.ID)
	if err != nil || !retainedProduct.Archived || retainedProduct.IsAvailable || retainedProduct.Image != uri || retainedProduct.Quantity != 2 {
		t.Fatalf("product not retained: %+v %v", retainedProduct, err)
	}
	retainedStyle, err := FindHairstyleByIDResult(ctx, style.ID)
	if err != nil || !retainedStyle.Archived || retainedStyle.IsAvailable || retainedStyle.Image != uri || retainedStyle.Video != style.Video {
		t.Fatalf("style not retained: %+v %v", retainedStyle, err)
	}
	if _, err := FindProductOwnedBy(ctx, product.ID, seller.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("archived product editable", err)
	}
	if _, err := FindHairstyleOwnedBy(ctx, style.ID, seller.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("archived style editable", err)
	}
	for _, action := range []func() error{func() error { return UpdateProduct(product) }, func() error { return UpdateHairstyle(style) }, func() error { _, err := DeleteProduct(product.ID, seller.ID); return err }, func() error { _, err := DeleteHairstyle(style.ID, seller.ID); return err }} {
		if err := action(); !errors.Is(err, ErrNotFound) {
			t.Fatal("stale edit/archive reopened item", err)
		}
	}
	ownProducts, err := ListProductsByBarber(ctx, seller.ID)
	if err != nil || len(ownProducts) != 0 {
		t.Fatal("archived product in seller catalogue", err)
	}
	ownStyles, err := ListHairstylesByBarberResult(ctx, seller.ID)
	if err != nil || len(ownStyles) != 0 {
		t.Fatal("archived style in seller catalogue", err)
	}
	availableProducts, err := ListAvailableProducts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range availableProducts {
		if p.ID == product.ID {
			t.Fatal("archived product available")
		}
	}
	availableStyles, err := ListAvailableHairstylesByBarberResult(ctx, seller.ID)
	if err != nil || len(availableStyles) != 0 {
		t.Fatal("archived style bookable", err)
	}
	if _, err := AddToCart(buyer.ID, product.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatal("archived product added to cart", err)
	}
	if err := SetCartQuantity(ctx, buyer.ID, product.ID, 2); !errors.Is(err, ErrUnavailable) {
		t.Fatal("archived cart quantity increased", err)
	}
	if _, err := CheckoutCart(buyer); err == nil {
		t.Fatal("archived product sold")
	}
	cart, err := ListCartItems(ctx, buyer.ID)
	if err != nil || len(cart) != 1 || cart[0].Quantity != 1 {
		t.Fatal("failed checkout lost recoverable cart", err)
	}
	if err := SetCartQuantity(ctx, buyer.ID, product.ID, 0); err != nil {
		t.Fatal("archived cart line cannot be removed", err)
	}
	if _, err := CreateBooking(newBooking(buyer, seller.ID, style.ID)); !errors.Is(err, ErrUnavailable) {
		t.Fatal("archived style booked", err)
	}
	storedBooking, err := FindBookingByIDResult(ctx, booking.ID)
	if err != nil || storedBooking.ServiceName != booking.ServiceName || storedBooking.ServiceDuration != booking.ServiceDuration {
		t.Fatal("booking history lost", err)
	}
	if err := SetBookingStatus(booking.ID, models.StatusAccepted, seller.ID, 0); err != nil {
		t.Fatal("archive broke existing booking", err)
	}
	if err := SetBookingStatus(booking.ID, models.StatusCompleted, seller.ID, 0); err != nil {
		t.Fatal("archive broke existing completion", err)
	}
	if err := FulfillOrderItem(ctx, items[0].ID, seller.ID, models.FulfillmentCancelled); err != nil {
		t.Fatal("archive broke existing cancellation", err)
	}
	retainedProduct, err = FindProductByIDResult(ctx, product.ID)
	if err != nil || retainedProduct.Quantity != 3 || !retainedProduct.Archived {
		t.Fatal("cancelled archived stock was lost", err)
	}
	retainedItems, err := ListProductOrderItems(ctx, order.ID)
	if err != nil || len(retainedItems) != 1 || retainedItems[0].ProductName != items[0].ProductName || retainedItems[0].SellerName != items[0].SellerName {
		t.Fatal("order history lost", err)
	}
	dueMedia(t, uri)
	called := false
	found, err := ProcessMediaCleanup(ctx, func(context.Context, string) error { called = true; return nil })
	if err != nil || !found || called || mediaState(t, uri) != "active" {
		t.Fatal("archive lost referenced media", err)
	}
	if databaseEnabled() {
		for _, attempt := range []struct {
			query string
			id    int
		}{
			{"update public.products set is_archived=false where id=$1", product.ID},
			{"update public.products set is_available=true where id=$1", product.ID},
			{"update public.hairstyles set is_archived=false where id=$1", style.ID},
			{"update public.hairstyles set is_available=true where id=$1", style.ID},
		} {
			if _, err := db.Exec(attempt.query, attempt.id); err == nil {
				t.Fatal("SQL reopened archived item", attempt.query)
			}
		}

	}
}

func TestCatalogueArchiveRaces(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	checkCatalogueArchiveRaces(t)
}
func checkCatalogueArchiveRaces(t *testing.T) {
	t.Helper()
	type result struct {
		archive bool
		err     error
	}
	t.Run("checkout serializes with product archive", func(t *testing.T) {
		seller, buyer, _ := postgresBookingFixture(t)
		product := postgresProduct(t, seller.ID, 3)
		if _, err := AddToCart(buyer.ID, product.ID); err != nil {
			t.Fatal(err)
		}
		outcomes := make(chan result, 2)
		go func() { _, err := CheckoutCart(buyer); outcomes <- result{err: err} }()
		go func() { _, err := DeleteProduct(product.ID, seller.ID); outcomes <- result{archive: true, err: err} }()
		sold := false
		for range 2 {
			r := <-outcomes
			if r.archive {
				if r.err != nil {
					t.Fatal("archive failed", r.err)
				}
			} else if r.err == nil {
				sold = true
			} else {
				var unavailable CheckoutError
				if !errors.As(r.err, &unavailable) {
					t.Fatal("unexpected checkout error", r.err)
				}
			}
		}
		retained, err := FindProductByIDResult(context.Background(), product.ID)
		wantStock := 3
		wantOrders := 0
		if sold {
			wantStock = 2
			wantOrders = 1
		}
		if err != nil || !retained.Archived || retained.Quantity != wantStock {
			t.Fatal("racing archive lost stock", err)
		}
		orders, err := ListProductOrdersByCustomer(context.Background(), buyer.ID)
		if err != nil || len(orders) != wantOrders {
			t.Fatal("racing archive lost order", err)
		}
		if sold {
			items, err := ListProductOrderItems(context.Background(), orders[0].ID)
			if err != nil || len(items) != 1 || items[0].ProductName != product.Name {
				t.Fatal("racing archive lost purchase details", err)
			}
		}
	})
	t.Run("booking serializes with service archive", func(t *testing.T) {
		seller, buyer, style := postgresBookingFixture(t)
		outcomes := make(chan result, 2)
		go func() { _, err := CreateBooking(newBooking(buyer, seller.ID, style.ID)); outcomes <- result{err: err} }()
		go func() { _, err := DeleteHairstyle(style.ID, seller.ID); outcomes <- result{archive: true, err: err} }()
		booked := false
		for range 2 {
			r := <-outcomes
			if r.archive {
				if r.err != nil {
					t.Fatal("archive failed", r.err)
				}
			} else if r.err == nil {
				booked = true
			} else if !errors.Is(r.err, ErrUnavailable) {
				t.Fatal("unexpected booking error", r.err)
			}
		}
		retained, err := FindHairstyleByIDResult(context.Background(), style.ID)
		if err != nil || !retained.Archived {
			t.Fatal("racing archive lost service", err)
		}
		bookings, err := ListBookingsByCustomerResult(context.Background(), buyer.ID)
		want := 0
		if booked {
			want = 1
		}
		if err != nil || len(bookings) != want {
			t.Fatal("racing archive lost booking", err)
		}
		if booked && bookings[0].ServiceName != style.Name {
			t.Fatal("racing archive lost service details")
		}
	})
}
