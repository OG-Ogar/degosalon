package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestCommerceReadFailuresDoNotRenderEmptySuccess(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer := models.Customer{ID: 1}
	barber := models.Barber{ID: 1}
	s := sessionData{CSRFToken: "test"}
	for name, handler := range map[string]func(http.ResponseWriter, *http.Request){
		"catalogue": func(w http.ResponseWriter, r *http.Request) { CustomerProductsHandler(w, r, customer, s) },
		"cart":      func(w http.ResponseWriter, r *http.Request) { CustomerCartHandler(w, r, customer, s) },
		"order": func(w http.ResponseWriter, r *http.Request) {
			CustomerProductOrderConfirmationHandler(w, r, customer, s, "1")
		},
		"seller inventory": func(w http.ResponseWriter, r *http.Request) { ListProductsHandler(w, r, barber, s) },
		"seller sales":     func(w http.ResponseWriter, r *http.Request) { BarberDashboardHandler(w, r, barber, s) },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("failed read returned %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
