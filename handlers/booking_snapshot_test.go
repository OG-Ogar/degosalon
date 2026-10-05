package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestBookingConfirmationUsesOriginalServiceAndShop(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	barber, err := storage.CreateBarber(models.Barber{Email: "history_seller@example.com", FullName: "Original barber", ShopName: "Original shop"})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{Email: "history_buyer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: barber.ID, Name: "Original service", Price: 1500, Duration: 60, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	booking, err := storage.CreateBooking(models.Booking{BarberID: barber.ID, CustomerID: customer.ID, HairstyleID: style.ID, Date: "2030-01-01", Time: "10:00", Status: models.StatusPending})
	if err != nil {
		t.Fatal(err)
	}
	style.Name = "Replacement service"
	style.Price = 99999
	style.Duration = 10
	if err := storage.UpdateHairstyle(style); err != nil {
		t.Fatal(err)
	}
	barber.ShopName = "Replacement shop"
	if err := storage.UpdateBarberProfile(context.Background(), barber); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	BookingConfirmationHandler(response, httptest.NewRequest(http.MethodGet, "/customer/booking-confirmation/"+strconv.Itoa(booking.ID), nil), customer, sessionData{}, strconv.Itoa(booking.ID))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Original service") || !strings.Contains(response.Body.String(), "Original shop") || strings.Contains(response.Body.String(), "Replacement service") {
		t.Fatalf("confirmation changed: %d %s", response.Code, response.Body.String())
	}
	row, err := describeBooking(context.Background(), booking)
	if err != nil || row.HairstyleName != "Original service" || row.ShopName != "Original shop" || row.Price != 1500 || row.Duration != 60 {
		t.Fatalf("history row changed: %+v %v", row, err)
	}
}
