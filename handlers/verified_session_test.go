package handlers

import (
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifiedSessionRejectsChangedPasswordAndSuspension(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	checkVerifiedSessionCredentials(t)
}

func checkVerifiedSessionCredentials(t *testing.T) {
	customer, err := storage.CreateCustomer(models.Customer{Email: "verified_customer@example.com", PasswordHash: "oldhash"})
	if err != nil {
		t.Fatal(err)
	}
	barber, err := storage.CreateBarber(models.Barber{Email: "verified_barber@example.com", PasswordHash: "oldhash"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		role   session.Role
		id     int
		change func(context.Context, int, string) error
	}{{session.RoleCustomer, customer.ID, storage.SetCustomerPasswordContext}, {session.RoleBarber, barber.ID, storage.SetBarberPasswordContext}} {
		t.Run(string(test.role), func(t *testing.T) {
			if err := test.change(context.Background(), test.id, "newhash"); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/login", nil)
			if err := startVerifiedAccountSession(w, r, test.role, test.id, "oldhash"); !errors.Is(err, session.ErrCredentialsChanged) || len(w.Result().Cookies()) != 0 {
				t.Fatalf("stale credentials: %v", err)
			}
			w = httptest.NewRecorder()
			if err := startVerifiedAccountSession(w, r, test.role, test.id, "newhash"); err != nil || len(w.Result().Cookies()) != 1 {
				t.Fatalf("current credentials: %v", err)
			}
			for _, cookie := range w.Result().Cookies() {
				session.Delete(cookie.Value)
			}
		})
	}
	if err := storage.SetBarberStatusContext(context.Background(), barber.ID, models.BarberSuspended); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	if err := startVerifiedAccountSession(w, httptest.NewRequest(http.MethodPost, "/login", nil), session.RoleBarber, barber.ID, "newhash"); !errors.Is(err, session.ErrCredentialsChanged) || len(w.Result().Cookies()) != 0 {
		t.Fatalf("suspended account: %v", err)
	}
}
