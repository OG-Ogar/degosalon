package storage

import (
	"barber-booking/internal/testdb"
	"barber-booking/models"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func checkBookingOrderPages(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	seller := newBarber(t, "page-seller@example.test")
	buyer := newCustomer(t, "page-buyer@example.test")
	other := newCustomer(t, "other-page-buyer@example.test")
	style := newHairstyle(t, seller.ID)
	product := postgresProduct(t, seller.ID, 40)
	var created []models.Booking
	for i := 0; i < 28; i++ {
		request := newBooking(buyer, seller.ID, style.ID)
		request.Date = time.Now().AddDate(0, 0, 7+i).Format("2006-01-02")
		booking, err := CreateBooking(request)
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, booking)
		if i%2 == 0 {
			if err := TransitionBookingStatus(booking.ID, models.StatusAccepted, []string{models.StatusPending}, seller.ID, 0); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := AddToCart(buyer.ID, product.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := CheckoutCartWithKey(ctx, buyer, fmt.Sprintf("%032x", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	mutate := func(id int, date, zone string, legacy bool) {
		t.Helper()
		if databaseEnabled() {
			_, err := db.ExecContext(ctx, "update public.bookings set booking_date=$2,booking_timezone=$3 where id=$1", id, date, zone)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			mu.Lock()
			defer mu.Unlock()
			for i := range bookings {
				if bookings[i].ID == id {
					bookings[i].Date = date
					bookings[i].BookingTimezone = zone
					if legacy {
						bookings[i].ServiceDuration = 0
					}
				}
			}
		}
	}
	mutate(created[0].ID, time.Now().AddDate(0, 0, -1).Format("2006-01-02"), "UTC", false)
	mutate(created[1].ID, created[1].Date, "invalid/timezone", false)
	mutate(created[2].ID, created[2].Date, "", true)
	f := BookingFilter{CustomerID: buyer.ID, Now: time.Now(), LegacyTimezone: "Africa/Lagos"}
	first, err := BookingPage(ctx, f, PageCursor{})
	if err != nil || len(first.Items) != 24 || !first.HasNext {
		t.Fatalf("first %+v %v", first, err)
	}
	next, err := BookingPage(ctx, f, PageCursor{Before: first.Items[23].ID})
	if err != nil || len(next.Items) != 4 || !next.HasPrevious || next.HasNext {
		t.Fatalf("next %+v %v", next, err)
	}
	if next.Items[1].HairstyleName != style.Name || next.Items[1].Duration != style.Duration {
		t.Fatal("legacy joined metadata missing")
	}
	back, err := BookingPage(ctx, f, PageCursor{After: next.Items[0].ID})
	if err != nil || len(back.Items) != 24 || back.Items[0].ID != first.Items[0].ID {
		t.Fatal("back page")
	}
	f.Group = "followup"
	follow, err := BookingPage(ctx, f, PageCursor{})
	if err != nil || len(follow.Items) != 2 {
		t.Fatalf("followup %+v %v", follow, err)
	}
	f.Group = "upcoming"
	upcoming, err := BookingPage(ctx, f, PageCursor{Limit: 6})
	if err != nil || len(upcoming.Items) != 6 || !upcoming.HasNext {
		t.Fatalf("preview %+v %v", upcoming, err)
	}
	counts, err := DashboardTotals(ctx, f)
	if err != nil || counts.Pending != 14 || counts.Upcoming != 26 || counts.AcceptedUpcoming != 13 {
		t.Fatalf("totals %+v %v", counts, err)
	}
	f.CustomerID = other.ID
	empty, err := BookingPage(ctx, f, PageCursor{})
	if err != nil || len(empty.Items) != 0 {
		t.Fatal("other customer rows exposed")
	}
	orders, err := CustomerOrderPage(ctx, buyer.ID, PageCursor{})
	if err != nil || len(orders.Items) != 24 || !orders.HasNext {
		t.Fatal("orders first")
	}
	older, err := CustomerOrderPage(ctx, buyer.ID, PageCursor{Before: orders.Items[23].ID})
	if err != nil || len(older.Items) != 4 {
		t.Fatal("orders next")
	}
	items, err := SellerOrderItemPage(ctx, seller.ID, "Pending", PageCursor{})
	if err != nil || len(items.Items) != 24 || !items.HasNext || items.Items[0].CustomerName != buyer.FullName {
		t.Fatalf("items %+v %v", items, err)
	}
	collected, err := SellerOrderItemPage(ctx, seller.ID, "Collected", PageCursor{})
	if err != nil || len(collected.Items) != 0 {
		t.Fatal("fulfillment filter")
	}
}
func TestBookingOrderPages(t *testing.T) { Reset(); t.Cleanup(Reset); checkBookingOrderPages(t) }
func TestPostgresBookingOrderPages(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkBookingOrderPages(t)
}
func TestHistoryQueriesBoundedAndOutages(t *testing.T) {
	var query string
	var args []driver.NamedValue
	connection := sql.OpenDB(scopedConnector{&query, &args})
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	for _, read := range []func() error{func() error {
		_, err := BookingPage(context.Background(), BookingFilter{CustomerID: 42}, PageCursor{Before: 99})
		return err
	}, func() error {
		_, err := CustomerOrderPage(context.Background(), 42, PageCursor{Before: 99})
		return err
	}, func() error {
		_, err := SellerOrderItemPage(context.Background(), 42, "all", PageCursor{Before: 99})
		return err
	}} {
		if err := read(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "limit $3") || len(args) != 3 || args[0].Value != int64(42) || args[2].Value != int64(25) {
			t.Fatalf("unbounded query %s %v", query, args)
		}
	}
	failure := errors.New("history outage")
	broken := sql.OpenDB(accountConnector{queryErr: failure})
	db = broken
	t.Cleanup(func() { broken.Close() })
	if _, err := DashboardTotals(context.Background(), BookingFilter{CustomerID: 42}); !errors.Is(err, failure) {
		t.Fatal("totals outage hidden")
	}
	if _, err := BookingPage(context.Background(), BookingFilter{CustomerID: 42}, PageCursor{}); !errors.Is(err, failure) {
		t.Fatal("history outage hidden")
	}
}
