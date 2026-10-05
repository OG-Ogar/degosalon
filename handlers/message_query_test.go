package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestMessageReadFailuresDoNotRenderEmptyConversations(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer := models.Customer{ID: 1}
	barber := models.Barber{ID: 1}
	s := sessionData{CSRFToken: "test"}
	for name, handler := range map[string]func(http.ResponseWriter, *http.Request){
		"customer summaries": func(w http.ResponseWriter, r *http.Request) { CustomerChatsHandler(w, r, customer, s) },
		"barber summaries":   func(w http.ResponseWriter, r *http.Request) { BarberChatsHandler(w, r, barber, s) },
		"customer thread":    func(w http.ResponseWriter, r *http.Request) { renderCustomerChat(w, r, s, customer, barber, "") },
		"barber thread":      func(w http.ResponseWriter, r *http.Request) { renderChat(w, r, s, barber, customer, "") },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("failed messages returned %d", response.Code)
			}
		})
	}
}
