package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestBookingDiscoveryAndAdminReadFailuresReturnServerErrors(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer := models.Customer{ID: 1}
	barber := models.Barber{ID: 1}
	s := sessionData{CSRFToken: "test"}
	for name, handler := range map[string]func(http.ResponseWriter, *http.Request){
		"public discovery":     ExploreBarbersHandler,
		"public profile":       func(w http.ResponseWriter, r *http.Request) { ExploreBarberHandler(w, r, "1") },
		"customer discovery":   func(w http.ResponseWriter, r *http.Request) { BrowseBarbersHandler(w, r, customer, s) },
		"customer profile":     func(w http.ResponseWriter, r *http.Request) { ViewBarberHandler(w, r, customer, s, "1") },
		"customer schedule":    func(w http.ResponseWriter, r *http.Request) { CustomerBookingsHandler(w, r, customer, s) },
		"barber schedule":      func(w http.ResponseWriter, r *http.Request) { BarberBookingsHandler(w, r, barber, s) },
		"booking confirmation": func(w http.ResponseWriter, r *http.Request) { BookingConfirmationHandler(w, r, customer, s, "1") },
		"services":             func(w http.ResponseWriter, r *http.Request) { ListHairstylesHandler(w, r, barber, s) },
		"admin dashboard":      func(w http.ResponseWriter, r *http.Request) { AdminDashboardHandler(w, r, s) },
		"admin bookings":       func(w http.ResponseWriter, r *http.Request) { AdminBookingsHandler(w, r, s) },
		"admin customers":      func(w http.ResponseWriter, r *http.Request) { AdminCustomersHandler(w, r, s) },
		"admin barbers":        func(w http.ResponseWriter, r *http.Request) { ListBarbersHandler(w, r, s) },
		"admin barber profile": func(w http.ResponseWriter, r *http.Request) { BarberProfileHandler(w, r, s, "1") },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("got %d: %s", response.Code, response.Body.String())
			}
		})
	}
	for name, handler := range map[string]func(http.ResponseWriter, *http.Request){
		"cancel": func(w http.ResponseWriter, r *http.Request) { CancelBookingHandler(w, r, customer, s, "1") },
		"accept": func(w http.ResponseWriter, r *http.Request) {
			BarberBookingActionHandler(w, r, barber, s, "1", "accept")
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			response := httptest.NewRecorder()
			handler(response, authFormRequest("/", url.Values{"csrf_token": {"test"}}).WithContext(ctx))
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
