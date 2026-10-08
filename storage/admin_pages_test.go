package storage

import (
	"barber-booking/internal/testdb"
	"barber-booking/models"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func checkAdminPages(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	var sellers []models.Barber
	var buyers []models.Customer
	for i := 0; i < 26; i++ {
		sellers = append(sellers, newBarber(t, fmt.Sprintf("admin-seller-%02d@example.test", i)))
		buyers = append(buyers, newCustomer(t, fmt.Sprintf("admin-buyer-%02d@example.test", i)))
	}
	style := newHairstyle(t, sellers[0].ID)
	if _, err := CreateBooking(newBooking(buyers[0], sellers[0].ID, style.ID)); err != nil {
		t.Fatal(err)
	}
	first, err := AdminBarberPage(ctx, "", PageCursor{})
	if err != nil || len(first.Items) != 24 || first.HasPrevious || !first.HasNext {
		t.Fatalf("first %+v %v", first, err)
	}
	next, err := AdminBarberPage(ctx, "", PageCursor{Before: first.Items[23].ID})
	if err != nil || len(next.Items) != 2 || !next.HasPrevious || next.HasNext {
		t.Fatalf("next %+v %v", next, err)
	}
	if next.Items[1].ID != sellers[0].ID || next.Items[1].Hairstyles != 1 || next.Items[1].Bookings != 1 {
		t.Fatal("account aggregates lost")
	}
	back, err := AdminBarberPage(ctx, "", PageCursor{After: next.Items[0].ID})
	if err != nil || len(back.Items) != 24 || back.Items[0].ID != first.Items[0].ID {
		t.Fatalf("back %+v %v", back, err)
	}
	match, err := AdminBarberPage(ctx, "ADMIN-SELLER-00", PageCursor{})
	if err != nil || len(match.Items) != 1 || match.Items[0].PasswordHash != "" {
		t.Fatalf("global search %+v %v", match, err)
	}
	customers, err := AdminCustomerPage(ctx, "", PageCursor{})
	if err != nil || len(customers.Items) != 24 || !customers.HasNext {
		t.Fatalf("customers %+v %v", customers, err)
	}
	older, err := AdminCustomerPage(ctx, "", PageCursor{Before: customers.Items[23].ID})
	if err != nil || len(older.Items) != 2 || older.Items[1].Bookings != 1 {
		t.Fatalf("older customers %+v %v", older, err)
	}
	customer, err := AdminCustomerPage(ctx, "ADMIN-BUYER-00", PageCursor{})
	if err != nil || len(customer.Items) != 1 || customer.Items[0].PasswordHash != "" {
		t.Fatalf("customer search %+v %v", customer, err)
	}
	counts, err := AdminDashboardCounts(ctx)
	if err != nil || counts != (AdminCounts{Barbers: 26, ActiveBarbers: 26, Customers: 26, Bookings: 1, PendingBookings: 1}) {
		t.Fatalf("counts %+v %v", counts, err)
	}
	empty, err := AdminBarberPage(ctx, "absent account", PageCursor{})
	if err != nil || len(empty.Items) != 0 {
		t.Fatal("empty search")
	}
	if _, err := AdminCustomerPage(ctx, strings.Repeat("x", 201), PageCursor{}); err == nil {
		t.Fatal("oversized search")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := AdminDashboardCounts(canceled); err == nil {
		t.Fatal("ignored cancellation")
	}
}
func TestAdminPages(t *testing.T) { Reset(); t.Cleanup(Reset); checkAdminPages(t) }
func TestPostgresAdminPages(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkAdminPages(t)
}
