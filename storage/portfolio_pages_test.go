package storage

import (
	"barber-booking/internal/testdb"
	"barber-booking/models"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

func checkPortfolioPages(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	seller := newBarber(t, "portfolio-seller@example.test")
	other := newBarber(t, "other-portfolio@example.test")
	buyer := newCustomer(t, "portfolio-buyer@example.test")
	for i := 0; i < 26; i++ {
		style := newHairstyle(t, seller.ID)
		if _, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Product", Price: 25, Quantity: 2, IsAvailable: true}); err != nil {
			t.Fatal(err)
		}
		request := newBooking(buyer, seller.ID, style.ID)
		request.Date = time.Now().AddDate(0, 0, 7+i).Format("2006-01-02")
		booking, err := CreateBooking(request)
		if err != nil {
			t.Fatal(err)
		}
		if err := TransitionBookingStatus(booking.ID, models.StatusAccepted, []string{models.StatusPending}, seller.ID, 0); err != nil {
			t.Fatal(err)
		}
		if err := TransitionBookingStatus(booking.ID, models.StatusCompleted, []string{models.StatusAccepted}, seller.ID, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateReview(ctx, models.Review{CustomerID: buyer.ID, BarberID: seller.ID, Rating: 5, Comment: "Review"}); err != nil {
			t.Fatal(err)
		}
	}
	newHairstyle(t, other.ID)
	hidden := newHairstyle(t, seller.ID)
	hidden.IsAvailable = false
	if err := UpdateHairstyleContext(ctx, hidden); err != nil {
		t.Fatal(err)
	}
	archived := newHairstyle(t, seller.ID)
	if _, err := DeleteHairstyleContext(ctx, archived.ID, seller.ID); err != nil {
		t.Fatal(err)
	}
	style, err := HairstylePage(ctx, seller.ID, true, PageCursor{})
	if err != nil || len(style.Items) != 24 || !style.HasNext {
		t.Fatalf("styles %+v %v", style, err)
	}
	next, err := HairstylePage(ctx, seller.ID, true, PageCursor{After: style.Items[23].ID})
	if err != nil || len(next.Items) != 2 || next.HasNext || !next.HasPrevious {
		t.Fatalf("next styles %+v %v", next, err)
	}
	back, err := HairstylePage(ctx, seller.ID, true, PageCursor{Before: next.Items[0].ID})
	if err != nil || len(back.Items) != 24 || back.Items[0].ID != style.Items[0].ID {
		t.Fatal("back styles")
	}
	owned, err := HairstylePage(ctx, seller.ID, false, PageCursor{After: style.Items[23].ID})
	if err != nil || len(owned.Items) != 3 {
		t.Fatal("owner unavailable styles missing")
	}
	product, err := ProductPage(ctx, seller.ID, true, PageCursor{})
	if err != nil || len(product.Items) != 24 || !product.HasNext {
		t.Fatal("products")
	}
	productNext, err := ProductPage(ctx, seller.ID, true, PageCursor{After: product.Items[23].ID})
	if err != nil || len(productNext.Items) != 2 {
		t.Fatal("products next")
	}
	review, err := ReviewPage(ctx, seller.ID, PageCursor{})
	if err != nil || len(review.Items) != 24 || !review.HasNext {
		t.Fatal("reviews")
	}
	older, err := ReviewPage(ctx, seller.ID, PageCursor{Before: review.Items[23].ID})
	if err != nil || len(older.Items) != 2 || !older.HasPrevious || older.HasNext {
		t.Fatal("older reviews")
	}
	rating, count, err := ReviewSummaryResult(ctx, seller.ID)
	if err != nil || rating != 5 || count != 26 {
		t.Fatal("rating should cover every page")
	}
}
func TestPortfolioPages(t *testing.T) { Reset(); t.Cleanup(Reset); checkPortfolioPages(t) }
func TestPostgresPortfolioPages(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkPortfolioPages(t)
}

func TestPortfolioQueriesScopedBoundedAndFailures(t *testing.T) {
	var query string
	var args []driver.NamedValue
	connection := sql.OpenDB(scopedConnector{&query, &args})
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	for _, read := range []func() error{func() error { _, e := HairstylePage(context.Background(), 42, true, PageCursor{After: 99}); return e }, func() error { _, e := ProductPage(context.Background(), 42, true, PageCursor{After: 99}); return e }, func() error { _, e := ReviewPage(context.Background(), 42, PageCursor{Before: 99}); return e }} {
		if err := read(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "barber_id=$1") || !strings.Contains(query, "limit $3") || len(args) != 3 || args[2].Value != int64(25) {
			t.Fatalf("unbounded page: %s %v", query, args)
		}
	}
	failure := errors.New("portfolio outage")
	broken := sql.OpenDB(accountConnector{queryErr: failure})
	db = broken
	t.Cleanup(func() { broken.Close() })
	if _, err := ReviewPage(context.Background(), 42, PageCursor{}); !errors.Is(err, failure) {
		t.Fatal("outage hidden")
	}
}
