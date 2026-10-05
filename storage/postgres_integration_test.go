package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"barber-booking/internal/testdb"
	"barber-booking/models"
)

func TestPostgresIntegrity(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	previous := db
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { current := db; db = previous; current.Close() })
	ctx := context.Background()

	t.Run("account writes preserve recovery and suspension", checkProfileEditsPreserveRecovery)
	t.Run("review ownership and concurrency", checkReviewIntegrity)
	t.Run("cart editing and removal", checkCartEditing)
	t.Run("checkout request replay", checkCheckoutReplay)

	t.Run("concurrent overlapping bookings", func(t *testing.T) {
		barber, customer, style := postgresBookingFixture(t)
		style.Duration = 60
		if err := UpdateHairstyle(style); err != nil {
			t.Fatal(err)
		}
		first := newBooking(customer, barber.ID, style.ID)
		second := first
		second.Time = "10:30"
		outcomes := concurrentErrors(func() error { _, e := CreateBooking(first); return e }, func() error { _, e := CreateBooking(second); return e })
		assertOneSuccess(t, outcomes, ErrSlotConflict)
		list, e := ListBookingsByBarberResult(ctx, barber.ID)
		if e != nil || len(list) != 1 {
			t.Fatalf("schedule: %+v %v", list, e)
		}
		adjacent := first
		adjacent.Time = "11:30"
		if _, e := CreateBooking(adjacent); e != nil {
			t.Fatalf("adjacent booking: %v", e)
		}
	})
	t.Run("guarded transition race", func(t *testing.T) {
		barber, customer, style := postgresBookingFixture(t)
		booking, e := CreateBooking(newBooking(customer, barber.ID, style.ID))
		if e != nil {
			t.Fatal(e)
		}
		outcomes := concurrentErrors(
			func() error {
				return TransitionBookingStatus(booking.ID, models.StatusAccepted, []string{models.StatusPending}, barber.ID, 0)
			},
			func() error {
				return TransitionBookingStatus(booking.ID, models.StatusRejected, []string{models.StatusPending}, barber.ID, 0)
			},
		)
		assertOneSuccess(t, outcomes, ErrInvalidTransition)
		stored, e := FindBookingByIDResult(ctx, booking.ID)
		if e != nil || (stored.Status != models.StatusAccepted && stored.Status != models.StatusRejected) {
			t.Fatalf("transition result: %+v %v", stored, e)
		}
	})
	t.Run("checkout failure rolls back earlier writes", func(t *testing.T) {
		barber, customer, _ := postgresBookingFixture(t)
		first := postgresProduct(t, barber.ID, 2)
		second := postgresProduct(t, barber.ID, 2)
		for _, product := range []models.Product{first, second} {
			if _, e := AddToCart(customer.ID, product.ID); e != nil {
				t.Fatal(e)
			}
		}
		// Fail the second item, after the first item and stock update were written.
		trigger := fmt.Sprintf("fail_checkout_%d", second.ID)
		function := trigger + "_fn"
		statement := fmt.Sprintf(`create function public.%s() returns trigger language plpgsql as $$ begin raise exception 'injected checkout failure'; end $$;
   create trigger %s before insert on public.product_order_items for each row when(new.product_id=%d) execute function public.%s()`, function, trigger, second.ID, function)
		if _, e := db.Exec(statement); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			if _, e := db.Exec(fmt.Sprintf("drop trigger %s on public.product_order_items; drop function public.%s()", trigger, function)); e != nil {
				t.Error(e)
			}
		})
		if _, e := CheckoutCart(customer); e == nil {
			t.Fatal("injected checkout failure succeeded")
		}
		orders, e := ListProductOrdersByCustomer(ctx, customer.ID)
		if e != nil || len(orders) != 0 {
			t.Fatalf("partial orders: %+v %v", orders, e)
		}
		count, e := CartItemCount(ctx, customer.ID)
		if e != nil || count != 2 {
			t.Fatalf("cart lost after failure: %d %v", count, e)
		}
		for _, product := range []models.Product{first, second} {
			stored, e := FindProductByIDResult(ctx, product.ID)
			if e != nil || stored.Quantity != 2 {
				t.Fatalf("stock changed after failure: %+v %v", stored, e)
			}
		}
	})
	t.Run("concurrent checkout creates one order", func(t *testing.T) {
		barber, customer, _ := postgresBookingFixture(t)
		product := postgresProduct(t, barber.ID, 2)
		if _, e := AddToCart(customer.ID, product.ID); e != nil {
			t.Fatal(e)
		}
		outcomes := concurrentErrors(func() error { _, e := CheckoutCart(customer); return e }, func() error { _, e := CheckoutCart(customer); return e })
		assertOneSuccess(t, outcomes, ErrNotFound)
		orders, e := ListProductOrdersByCustomer(ctx, customer.ID)
		if e != nil || len(orders) != 1 {
			t.Fatalf("duplicate checkout: %+v %v", orders, e)
		}
		items, e := ListProductOrderItems(ctx, orders[0].ID)
		if e != nil || len(items) != 1 || items[0].Quantity != 1 {
			t.Fatalf("order items: %+v %v", items, e)
		}
		stored, e := FindProductByIDResult(ctx, product.ID)
		if e != nil || stored.Quantity != 1 {
			t.Fatalf("stock: %+v %v", stored, e)
		}
	})
	t.Run("concurrent new cart line is never lost", func(t *testing.T) {
		barber, customer, _ := postgresBookingFixture(t)
		first := postgresProduct(t, barber.ID, 3)
		second := postgresProduct(t, barber.ID, 3)
		if _, err := AddToCart(customer.ID, first.ID); err != nil {
			t.Fatal(err)
		}
		// Delay order creation so an independent addition can overlap checkout.
		name := fmt.Sprintf("delay_cart_checkout_%d", customer.ID)
		statement := fmt.Sprintf(`create function public.%s() returns trigger language plpgsql as $$ begin perform pg_sleep(0.4); return new; end $$;
   create trigger %s before insert on public.product_orders for each row when(new.customer_id=%d) execute function public.%s()`, name, name, customer.ID, name)
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := db.Exec(fmt.Sprintf("drop trigger %s on public.product_orders; drop function public.%s()", name, name)); err != nil {
				t.Error(err)
			}
		})
		checkout := make(chan error, 1)
		go func() { _, err := CheckoutCartContext(ctx, customer); checkout <- err }()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var sleeping bool
			err := db.QueryRow(`select exists(select 1 from pg_stat_activity where pid<>pg_backend_pid() and wait_event='PgSleep' and query like $1)`, "insert into public.product_orders%").Scan(&sleeping)
			if err != nil {
				t.Fatal(err)
			}
			if sleeping {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("checkout did not reach injected delay")
			}
			time.Sleep(5 * time.Millisecond)
		}
		_, addErr := AddToCartContext(ctx, customer.ID, second.ID)
		checkoutErr := <-checkout
		if addErr != nil || checkoutErr != nil {
			t.Fatalf("concurrent writes: add=%v checkout=%v", addErr, checkoutErr)
		}
		cart, err := ListCartItems(ctx, customer.ID)
		if err != nil || len(cart) != 1 || cart[0].ProductID != second.ID || cart[0].Quantity != 1 {
			t.Fatalf("new cart line lost: %+v %v", cart, err)
		}
	})
	t.Run("competing customers cannot oversell", func(t *testing.T) {
		barber, customer, _ := postgresBookingFixture(t)
		other := newCustomer(t, "other_"+strings.ReplaceAll(t.Name(), "/", "_")+"@example.com")
		product := postgresProduct(t, barber.ID, 1)
		for _, buyer := range []models.Customer{customer, other} {
			if _, e := AddToCart(buyer.ID, product.ID); e != nil {
				t.Fatal(e)
			}
		}
		outcomes := concurrentErrors(func() error { _, e := CheckoutCart(customer); return e }, func() error { _, e := CheckoutCart(other); return e })
		successes := 0
		for _, e := range outcomes {
			if e == nil {
				successes++
				continue
			}
			var conflict CheckoutError
			if !errors.As(e, &conflict) {
				t.Fatalf("unexpected checkout error: %v", e)
			}
		}
		if successes != 1 {
			t.Fatalf("checkout successes=%d", successes)
		}
		stored, e := FindProductByIDResult(ctx, product.ID)
		if e != nil || stored.Quantity != 0 {
			t.Fatalf("stock: %+v %v", stored, e)
		}
		first, e := CartItemCount(ctx, customer.ID)
		if e != nil {
			t.Fatal(e)
		}
		second, e := CartItemCount(ctx, other.ID)
		if e != nil || first+second != 1 {
			t.Fatalf("failed checkout cart not preserved: %d %d %v", first, second, e)
		}
	})
	t.Run("order history survives storage reconnect", func(t *testing.T) {
		barber, customer, _ := postgresBookingFixture(t)
		product := postgresProduct(t, barber.ID, 1)
		if _, e := AddToCart(customer.ID, product.ID); e != nil {
			t.Fatal(e)
		}
		order, e := CheckoutCart(customer)
		if e != nil {
			t.Fatal(e)
		}
		if e := db.Close(); e != nil {
			t.Fatal(e)
		}
		db, e = sql.Open("pgx", databaseURL)
		if e != nil {
			t.Fatal(e)
		}
		orders, e := ListProductOrdersByCustomer(ctx, customer.ID)
		if e != nil || len(orders) != 1 || orders[0].ID != order.ID {
			t.Fatalf("history after reconnect: %+v %v", orders, e)
		}
	})
	t.Run("checkout replay survives storage reconnect", func(t *testing.T) {
		barber, customer, _ := postgresBookingFixture(t)
		product := postgresProduct(t, barber.ID, 2)
		if _, err := AddToCart(customer.ID, product.ID); err != nil {
			t.Fatal(err)
		}
		key := "abcdef0123456789abcdef0123456789"
		first, err := CheckoutCartWithKey(ctx, customer, key)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db, err = sql.Open("pgx", databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := CheckoutCartWithKey(ctx, customer, key)
		if err != nil || replay.ID != first.ID {
			t.Fatalf("persistent replay: %+v %v", replay, err)
		}
	})
	t.Run("scoped message SQL returns latest and count", func(t *testing.T) {
		barber, customer, _ := postgresBookingFixture(t)
		other := newCustomer(t, "other_"+strings.ReplaceAll(t.Name(), "/", "_")+"@example.com")
		for i, buyer := range []models.Customer{customer, other, customer} {
			if _, e := AddMessage(ctx, models.Message{BarberID: barber.ID, CustomerID: buyer.ID, SenderRole: models.SenderCustomer, SenderName: buyer.FullName, Text: fmt.Sprintf("message %d", i), SentAt: time.Unix(int64(i+1), 0)}); e != nil {
				t.Fatal(e)
			}
		}
		list, e := ConversationsForCustomer(ctx, customer.ID)
		if e != nil || len(list) != 1 || list[0].CustomerID != customer.ID || list[0].LastText != "message 2" || list[0].Count != 2 {
			t.Fatalf("customer messages: %+v %v", list, e)
		}
		list, e = ConversationsForBarber(ctx, barber.ID)
		if e != nil || len(list) != 2 || list[0].LastText != "message 2" {
			t.Fatalf("barber messages: %+v %v", list, e)
		}
	})
	t.Run("referenced services cannot be erased", func(t *testing.T) {
		barber, customer, style := postgresBookingFixture(t)
		if _, e := CreateBooking(newBooking(customer, barber.ID, style.ID)); e != nil {
			t.Fatal(e)
		}
		if _, e := DeleteHairstyle(style.ID, barber.ID); e == nil || errors.Is(e, ErrNotFound) {
			t.Fatalf("deletion must preserve foreign-key conflict: %v", e)
		}
	})
	t.Run("public tables retain RLS", func(t *testing.T) {
		var count int
		if e := db.QueryRow("select count(*) from pg_tables where schemaname='public' and not rowsecurity").Scan(&count); e != nil {
			t.Fatal(e)
		}
		if count != 0 {
			t.Fatalf("public tables without RLS=%d", count)
		}
	})
}

func postgresBookingFixture(t *testing.T) (models.Barber, models.Customer, models.Hairstyle) {
	t.Helper()
	suffix := strings.ReplaceAll(t.Name(), "/", "_")
	barber := newBarber(t, "barber_"+suffix+"@example.com")
	customer := newCustomer(t, "customer_"+suffix+"@example.com")
	return barber, customer, newHairstyle(t, barber.ID)
}
func postgresProduct(t *testing.T, barberID, quantity int) models.Product {
	t.Helper()
	product, e := CreateProduct(models.Product{BarberID: barberID, Name: "Product", Category: "Care", Price: 1250, Quantity: quantity, IsAvailable: true})
	if e != nil {
		t.Fatal(e)
	}
	return product
}
func concurrentErrors(first, second func() error) []error {
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, action := range []func() error{first, second} {
		go func(action func() error) { <-start; results <- action() }(action)
	}
	close(start)
	return []error{<-results, <-results}
}
func assertOneSuccess(t *testing.T, outcomes []error, conflict error) {
	t.Helper()
	successes := 0
	for _, e := range outcomes {
		if e == nil {
			successes++
		} else if !errors.Is(e, conflict) {
			t.Fatalf("unexpected concurrent result: %v", e)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent successes=%d, want 1", successes)
	}
}
