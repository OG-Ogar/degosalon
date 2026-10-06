package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"barber-booking/internal/testdb"
	"barber-booking/models"
)

func TestDatabaseContextBounds(t *testing.T) {
	start := time.Now()
	ctx, cancel := databaseContext(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || deadline.Before(start.Add(DatabaseOperationTimeout-time.Second)) || deadline.After(start.Add(DatabaseOperationTimeout+time.Second)) {
		t.Fatal("missing operation bound")
	}
	parent, parentCancel := context.WithTimeout(context.Background(), time.Second)
	defer parentCancel()
	child, childCancel := databaseContext(parent)
	defer childCancel()
	want, _ := parent.Deadline()
	got, _ := child.Deadline()
	if !got.Equal(want) {
		t.Fatal("parent deadline extended")
	}
	parentCancel()
	if !errors.Is(child.Err(), context.Canceled) {
		t.Fatal("parent cancellation detached")
	}
}
func TestDatabasePoolConfiguration(t *testing.T) {
	cfg, err := poolConfig(func(string) string { return "" })
	if err != nil || cfg.open != 12 || cfg.idle != 6 || cfg.lifetime != 30*time.Minute || cfg.idleTime != 5*time.Minute {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	cfg, err = poolConfig(func(name string) string {
		switch name {
		case "DB_MAX_OPEN_CONNS":
			return "2"
		case "DB_CONN_MAX_LIFETIME":
			return "2m"
		}
		return ""
	})
	if err != nil || cfg.idle != 2 || cfg.idleTime != 2*time.Minute {
		t.Fatal("derived limits exceed configuration")
	}
	for _, test := range []struct{ key, value string }{
		{"DB_MAX_OPEN_CONNS", "private-invalid-setting"}, {"DB_MAX_OPEN_CONNS", "0"}, {"DB_MAX_OPEN_CONNS", "101"},
		{"DB_MAX_IDLE_CONNS", "13"}, {"DB_MAX_IDLE_CONNS", "-1"},
		{"DB_CONN_MAX_LIFETIME", "0"}, {"DB_CONN_MAX_LIFETIME", "30s"}, {"DB_CONN_MAX_LIFETIME", "25h"}, {"DB_CONN_MAX_IDLE_TIME", "31m"},
	} {
		_, err := poolConfig(func(name string) string {
			if name == test.key {
				return test.value
			}
			return ""
		})
		if err == nil || !strings.Contains(err.Error(), test.key) || strings.Contains(err.Error(), "private-invalid-setting") {
			t.Fatalf("unsafe or accepted configuration for %s: %v", test.key, err)
		}
	}
	for _, name := range []string{"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME"} {
		t.Setenv(name, "")
	}
	connection, err := sql.Open("pgx", "postgres://unused.invalid/example")
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := configurePool(connection); err != nil || connection.Stats().MaxOpenConnections != 12 {
		t.Fatal("pool limit not applied")
	}
}
func TestMutationContextsRejectCancellation(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := []func() error{
		func() error { _, err := CreateCustomerContext(ctx, models.Customer{}); return err },
		func() error { _, err := CreateBarberContext(ctx, models.Barber{}); return err },
		func() error { return UpdateCustomerContext(ctx, models.Customer{}) },
		func() error { return UpdateBarberContext(ctx, models.Barber{}) },
		func() error { _, err := CreateHairstyleContext(ctx, models.Hairstyle{}); return err },
		func() error { return UpdateHairstyleContext(ctx, models.Hairstyle{}) },
		func() error { _, err := DeleteHairstyleContext(ctx, 1, 1); return err },
		func() error { _, err := CreateProductContext(ctx, models.Product{}); return err },
		func() error { return UpdateProductContext(ctx, models.Product{}) },
		func() error { _, err := DeleteProductContext(ctx, 1, 1); return err },
		func() error { _, err := CreateProductOrderContext(ctx, models.ProductOrder{}); return err },
		func() error { _, err := CreateProductOrderItemContext(ctx, models.ProductOrderItem{}); return err },
		func() error { return ClearCartContext(ctx, 1) },
	}
	for i, call := range calls {
		if err := call(); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled mutation %d: %v", i, err)
		}
	}
	if len(customers)+len(barbers)+len(hairstyles)+len(products)+len(productOrders)+len(productOrderItems) != 0 {
		t.Fatal("cancelled mutation changed memory")
	}
}

func TestPostgresDatabaseBounds(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	previous := db
	connection, err := openDatabase(address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	seller, buyer, _ := postgresBookingFixture(t)
	product := postgresProduct(t, seller.ID, 5)
	t.Run("pool acquisition honors request deadline", func(t *testing.T) {
		connection.SetMaxOpenConns(1)
		held, err := connection.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err = ListProductsByBarber(ctx, seller.ID)
		held.Close()
		connection.SetMaxOpenConns(8)
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
			t.Fatal("pool acquisition was unbounded", err)
		}
	})
	t.Run("row mutation honors cancellation without writing", func(t *testing.T) {
		held, err := connection.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Rollback()
		if _, err := held.Exec(`select id from public.products where id=$1 for update`, product.ID); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		edited := product
		edited.Name = "Must not commit"
		start := time.Now()
		err = UpdateProductContext(ctx, edited)
		if err == nil || ctx.Err() == nil || time.Since(start) > time.Second {
			t.Fatal("blocked write was not cancelled", err)
		}
		held.Rollback()
		got, err := FindProductByIDResult(context.Background(), product.ID)
		if err != nil || got.Name != product.Name || got.Quantity != product.Quantity {
			t.Fatal("cancelled write changed product")
		}
	})
	t.Run("default bound cancels lock wait", func(t *testing.T) {
		held, err := connection.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Rollback()
		if _, err := held.Exec(`select id from public.products where id=$1 for update`, product.ID); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		err = UpdateProductContext(context.Background(), product)
		if err == nil || time.Since(start) < DatabaseOperationTimeout-time.Second || time.Since(start) > DatabaseOperationTimeout+2*time.Second {
			t.Fatal("default write bound missing", err)
		}
		held.Rollback()
	})
	t.Run("transaction cancellation preserves cart and stock", func(t *testing.T) {
		if _, err := AddToCart(buyer.ID, product.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := connection.Exec(`create function degosalon_internal.slow_order_commit() returns trigger language plpgsql as $$ begin perform pg_sleep(2);return new;end $$;create constraint trigger slow_order_commit after insert on public.product_orders deferrable initially deferred for each row execute function degosalon_internal.slow_order_commit()`); err != nil {
			t.Fatal(err)
		}
		defer connection.Exec(`drop trigger slow_order_commit on public.product_orders`)
		connection.SetMaxOpenConns(1)
		connection.SetMaxIdleConns(1)
		var backendPID int
		if err := connection.QueryRow("select pg_backend_pid()").Scan(&backendPID); err != nil {
			t.Fatal(err)
		}
		observer, err := sql.Open("pgx", address)
		if err != nil {
			t.Fatal(err)
		}
		defer observer.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err = CheckoutCartContext(ctx, buyer)
		if err == nil || ctx.Err() == nil || time.Since(start) > time.Second {
			t.Fatal("commit ignored deadline", err)
		}
		// Observe the original backend through a different connection. This waits
		// for a canceled COMMIT to settle, rather than reading an early MVCC snapshot
		// while a late deferred trigger could still commit the transaction.
		settled, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		for {
			var busy bool
			if err := observer.QueryRowContext(settled, `select exists(select 1 from pg_stat_activity where pid=$1 and state<>'idle')`, backendPID).Scan(&busy); err != nil {
				t.Fatal(err)
			}
			if !busy {
				break
			}
			timer := time.NewTimer(10 * time.Millisecond)
			select {
			case <-settled.Done():
				timer.Stop()
				t.Fatal("original commit did not settle")
			case <-timer.C:
			}
		}
		got, err := FindProductByIDResult(context.Background(), product.ID)
		if err != nil || got.Quantity != 5 {
			t.Fatal("cancelled commit consumed stock", err)
		}
		items, err := ListCartItems(context.Background(), buyer.ID)
		if err != nil || len(items) != 1 || items[0].Quantity != 1 {
			t.Fatal("cancelled commit cleared cart", err)
		}
		orders, err := ListProductOrdersByCustomer(context.Background(), buyer.ID)
		if err != nil || len(orders) != 0 {
			t.Fatal("cancelled commit persisted order", err)
		}
	})
}
