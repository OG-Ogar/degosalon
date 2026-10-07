package handlers

import (
	"barber-booking/models"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDomainStatusCatalog(t *testing.T) {
	domains := map[string][]string{
		"booking":     {models.StatusPending, models.StatusAccepted, models.StatusRejected, models.StatusCancelled, models.StatusCompleted},
		"account":     {models.BarberActive, models.BarberSuspended, models.BarberArchived},
		"order":       {models.OrderPending, models.OrderPaid, models.OrderReady, models.OrderCancelled, models.OrderCompleted},
		"fulfillment": {models.FulfillmentPending, models.FulfillmentReady, models.FulfillmentCollected, models.FulfillmentCancelled},
	}
	tones := map[string]bool{"warning": true, "positive": true, "danger": true, "muted": true, "info": true}
	for domain, values := range domains {
		for _, value := range values {
			status := domainStatus(domain, value)
			if status.Label != value || !tones[status.Tone] || status.Description == "" {
				t.Fatalf("unmapped state %s/%s: %+v", domain, value, status)
			}
		}
	}
	if domainStatus("booking", models.BarberActive).Label != "Unknown status" {
		t.Fatal("cross-domain status accepted")
	}
	for _, domain := range []string{"booking", "account", "order", "fulfillment", "missing"} {
		if s := domainStatus(domain, `bad" onclick="alert(1)`); s.Tone != "muted" || s.Label != "Unknown status" {
			t.Fatal("unsafe fallback", s)
		}
	}
}

func TestBookingDetailsUsesCurrentStatus(t *testing.T) {
	for _, status := range []string{models.StatusPending, models.StatusAccepted, models.StatusRejected, models.StatusCancelled, models.StatusCompleted, "future-state"} {
		rec := httptest.NewRecorder()
		render(rec, "customer/booking-confirmation.html", bookingConfirmationView{Booking: models.Booking{ID: 1, Status: status, Date: "2026-12-01", Time: "10:00"}, Barber: models.Barber{ID: 1, ShopName: "Shop"}, Hairstyle: models.Hairstyle{ID: 1, Name: "Cut", Price: 1000}})
		body := rec.Body.String()
		presentation := domainStatus("booking", status)
		if rec.Code != 200 || !strings.Contains(body, ">"+presentation.Label+"</span>") || !strings.Contains(body, "<p>"+presentation.Description+"</p>") {
			t.Fatalf("%s rendered incorrectly: %d %s", status, rec.Code, body)
		}
		if strings.Contains(body, "Pending Barber Approval") || strings.Contains(body, "Booking Request Sent") {
			t.Fatal("stale confirmation copy", status)
		}
	}
}
