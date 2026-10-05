package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"barber-booking/models"
)

func TestDiscoveryReadsPreserveDatabaseErrors(t *testing.T) {
	failure := errors.New("discovery database unavailable")
	for _, tc := range []struct {
		name      string
		connector accountConnector
		want      error
	}{
		{"query", accountConnector{queryErr: failure}, failure}, {"scan", accountConnector{invalidRow: true}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := db
			conn := sql.OpenDB(tc.connector)
			db = conn
			t.Cleanup(func() { db = previous; conn.Close() })
			for name, read := range map[string]func() error{
				"barbers":            func() error { _, e := ListBarbersResult(context.Background()); return e },
				"active barbers":     func() error { _, e := ListActiveBarbersResult(context.Background()); return e },
				"customers":          func() error { _, e := ListCustomersResult(context.Background()); return e },
				"services":           func() error { _, e := ListHairstylesByBarberResult(context.Background(), 1); return e },
				"available services": func() error { _, e := ListAvailableHairstylesByBarberResult(context.Background(), 1); return e },
				"bookings":           func() error { _, e := ListBookingsResult(context.Background()); return e },
				"customer bookings":  func() error { _, e := ListBookingsByCustomerResult(context.Background(), 1); return e },
				"barber bookings":    func() error { _, e := ListBookingsByBarberResult(context.Background(), 1); return e },
				"booking":            func() error { _, e := FindBookingByIDResult(context.Background(), 1); return e },
				"reviews":            func() error { _, e := ListReviewsByBarberResult(context.Background(), 1); return e },
				"rating":             func() error { _, _, e := ReviewSummaryResult(context.Background(), 1); return e },
				"eligibility":        func() error { _, e := CanCustomerReviewResult(context.Background(), 1, 1); return e },
				"review creation": func() error {
					_, e := CreateReview(context.Background(), models.Review{CustomerID: 1, BarberID: 1, Rating: 5})
					return e
				},
				"owned service": func() error { _, e := FindHairstyleOwnedBy(context.Background(), 1, 1); return e },
			} {
				t.Run(name, func(t *testing.T) {
					e := read()
					if e == nil || errors.Is(e, ErrNotFound) || errors.Is(e, ErrForbidden) {
						t.Fatalf("query failure hidden: %v", e)
					}
					if tc.want != nil && !errors.Is(e, tc.want) {
						t.Fatalf("got %v want %v", e, tc.want)
					}
				})
			}
		})
	}
}
