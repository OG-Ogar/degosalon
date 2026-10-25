package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

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
	style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: barber.ID, Name: "Original service", Price: 1500, Duration: 60, Image: "/uploads/original-style.jpg", IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	booking, err := storage.CreateBooking(models.Booking{BarberID: barber.ID, CustomerID: customer.ID, HairstyleID: style.ID, Date: time.Now().AddDate(0, 0, 7).Format("2006-01-02"), Time: "10:00", Status: models.StatusPending})
	if err != nil {
		t.Fatal(err)
	}
	style.Name = "Replacement service"
	style.Price = 99999
	style.Duration = 10
	style.Image = "/uploads/replacement-style.jpg"
	if err := storage.UpdateHairstyle(style); err != nil {
		t.Fatal(err)
	}
	barber.ShopName = "Replacement shop"
	if err := storage.UpdateBarberProfile(context.Background(), barber); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	BookingConfirmationHandler(response, httptest.NewRequest(http.MethodGet, "/customer/booking-confirmation/"+strconv.Itoa(booking.ID), nil), customer, sessionData{}, strconv.Itoa(booking.ID))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Original service") || !strings.Contains(response.Body.String(), "Original shop") || !strings.Contains(response.Body.String(), "/uploads/original-style.jpg") || strings.Contains(response.Body.String(), "Replacement service") {
		t.Fatalf("confirmation changed: %d %s", response.Code, response.Body.String())
	}
	row, err := describeBooking(context.Background(), booking)
	if err != nil || row.HairstyleName != "Original service" || row.HairstyleImage != "/uploads/original-style.jpg" || row.ShopName != "Original shop" || row.Price != 1500 || row.Duration != 60 {
		t.Fatalf("history row changed: %+v %v", row, err)
	}
}

func TestBookingConfirmationCollectsMissingPhone(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	barber, err := storage.CreateBarber(models.Barber{Email: "phone_barber@example.com", FullName: "Barber", ShopName: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{Email: "phone_customer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: barber.ID, Name: "Cut", Duration: 30, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	booking, err := storage.CreateBooking(models.Booking{BarberID: barber.ID, CustomerID: customer.ID, HairstyleID: style.ID, Date: time.Now().AddDate(0, 0, 7).Format("2006-01-02"), Time: "10:00"})
	if err != nil {
		t.Fatal(err)
	}
	path := "/customer/booking-confirmation/" + strconv.Itoa(booking.ID)

	get := httptest.NewRecorder()
	BookingConfirmationHandler(get, httptest.NewRequest(http.MethodGet, path, nil), customer, sessionData{CSRFToken: "test-csrf"}, strconv.Itoa(booking.ID))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "Add a phone number for your barber") {
		t.Fatalf("missing phone prompt: %d %s", get.Code, get.Body.String())
	}

	postPhone := func(phone string) *httptest.ResponseRecorder {
		form := url.Values{"csrf_token": {"test-csrf"}, "phone": {phone}}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		BookingConfirmationHandler(response, req, customer, sessionData{CSRFToken: "test-csrf"}, strconv.Itoa(booking.ID))
		return response
	}
	invalid := postPhone("bad-phone")
	if invalid.Code != http.StatusOK || !strings.Contains(invalid.Body.String(), "Enter a phone number") {
		t.Fatalf("invalid phone accepted: %d %s", invalid.Code, invalid.Body.String())
	}
	valid := postPhone("+234 801-234-5678")
	if valid.Code != http.StatusSeeOther || valid.Header().Get("Location") != path {
		t.Fatalf("valid phone not saved with redirect: %d %q", valid.Code, valid.Header().Get("Location"))
	}
	storedCustomer, err := storage.FindCustomerByIDResult(context.Background(), customer.ID)
	if err != nil {
		t.Fatal(err)
	}
	storedBooking, err := storage.FindBookingByIDResult(context.Background(), booking.ID)
	if err != nil || storedCustomer.PhoneNumber != "+234 801-234-5678" || storedBooking.CustomerPhone != "+234 801-234-5678" {
		t.Fatalf("phone not persisted: customer=%+v booking=%+v error=%v", storedCustomer, storedBooking, err)
	}
}
