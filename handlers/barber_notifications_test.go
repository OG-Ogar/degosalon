package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

type bookingAlertBody struct {
	Latest   int `json:"latest"`
	Bookings []struct {
		ID       int     `json:"id"`
		Customer string  `json:"customer"`
		Service  string  `json:"service"`
		Date     string  `json:"date"`
		Time     string  `json:"time"`
		Price    float64 `json:"price"`
	} `json:"bookings"`
}

func readBookingAlerts(t *testing.T, response *httptest.ResponseRecorder) bookingAlertBody {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var body bookingAlertBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode alerts: %v", err)
	}
	return body
}

func alertFixture(t *testing.T) (models.Barber, models.Customer, models.Hairstyle) {
	t.Helper()
	storage.Reset()
	t.Cleanup(storage.Reset)

	barber, err := storage.CreateBarber(models.Barber{
		Email:       "alerts_barber@example.com",
		FullName:    "Alerts Barber",
		ShopName:    "Alerts Cuts",
		ShopAddress: "1 Alert Street, Lagos",
		Status:      models.BarberActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{
		Email:       "alerts_customer@example.com",
		FullName:    "Ada Customer",
		PhoneNumber: "08012340000",
	})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{
		BarberID:    barber.ID,
		Name:        "Low Fade",
		Price:       5000,
		Duration:    30,
		IsAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return barber, customer, style
}

func newAlertBooking(t *testing.T, barber models.Barber, customer models.Customer, style models.Hairstyle, timeText string) models.Booking {
	t.Helper()
	booking, err := storage.CreateBooking(models.Booking{
		BarberID:      barber.ID,
		HairstyleID:   style.ID,
		CustomerID:    customer.ID,
		CustomerName:  customer.FullName,
		CustomerPhone: customer.PhoneNumber,
		CustomerEmail: customer.Email,
		Date:          time.Now().AddDate(0, 0, 5).Format("2006-01-02"),
		Time:          timeText,
		Status:        models.StatusPending,
		CreatedAt:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return booking
}

func TestBarberBookingAlertBaselineThenNewBooking(t *testing.T) {
	barber, customer, style := alertFixture(t)
	first := newAlertBooking(t, barber, customer, style, "10:00")

	// The first call is a baseline: it must not replay the pending booking.
	baseline := httptest.NewRecorder()
	BarberNotificationsHandler(baseline, httptest.NewRequest(http.MethodGet, "/barber/notifications", nil), barber)
	body := readBookingAlerts(t, baseline)
	if body.Latest != first.ID {
		t.Fatalf("baseline latest = %d, want %d", body.Latest, first.ID)
	}
	if len(body.Bookings) != 0 {
		t.Fatalf("baseline replayed %d bookings", len(body.Bookings))
	}

	second := newAlertBooking(t, barber, customer, style, "14:00")

	response := httptest.NewRecorder()
	BarberNotificationsHandler(response, httptest.NewRequest(http.MethodGet, "/barber/notifications?after="+strconv.Itoa(first.ID), nil), barber)
	body = readBookingAlerts(t, response)

	if body.Latest != second.ID {
		t.Fatalf("latest = %d, want %d", body.Latest, second.ID)
	}
	if len(body.Bookings) != 1 {
		t.Fatalf("got %d alerts, want 1", len(body.Bookings))
	}
	alert := body.Bookings[0]
	if alert.ID != second.ID {
		t.Errorf("alert id = %d, want %d", alert.ID, second.ID)
	}
	if alert.Customer != customer.FullName {
		t.Errorf("alert customer = %q, want %q", alert.Customer, customer.FullName)
	}
	if alert.Service != style.Name {
		t.Errorf("alert service = %q, want %q", alert.Service, style.Name)
	}
	if alert.Price != style.Price {
		t.Errorf("alert price = %v, want %v", alert.Price, style.Price)
	}
	if alert.Time != "10:00" {
		t.Errorf("alert time = %q, want 10:00", alert.Time)
	}
}

func TestBarberBookingAlertIgnoresOtherBarbers(t *testing.T) {
	barber, customer, _ := alertFixture(t)

	other, err := storage.CreateBarber(models.Barber{
		Email:    "other_barber@example.com",
		FullName: "Other Barber",
		Status:   models.BarberActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherStyle, err := storage.CreateHairstyle(models.Hairstyle{
		BarberID:    other.ID,
		Name:        "Other Cut",
		Price:       3000,
		Duration:    20,
		IsAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateBooking(models.Booking{
		BarberID:      other.ID,
		HairstyleID:   otherStyle.ID,
		CustomerID:    customer.ID,
		CustomerName:  customer.FullName,
		CustomerPhone: customer.PhoneNumber,
		CustomerEmail: customer.Email,
		Date:          time.Now().AddDate(0, 0, 5).Format("2006-01-02"),
		Time:          "11:00",
		Status:        models.StatusPending,
		CreatedAt:     time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	BarberNotificationsHandler(response, httptest.NewRequest(http.MethodGet, "/barber/notifications?after=0", nil), barber)
	body := readBookingAlerts(t, response)

	if len(body.Bookings) != 0 {
		t.Fatalf("leaked %d bookings from another barber", len(body.Bookings))
	}
	if body.Latest != 0 {
		t.Fatalf("latest = %d, want 0", body.Latest)
	}
}

func TestBarberBookingAlertRejectsBadCursor(t *testing.T) {
	barber, _, _ := alertFixture(t)

	response := httptest.NewRecorder()
	BarberNotificationsHandler(response, httptest.NewRequest(http.MethodGet, "/barber/notifications?after=abc", nil), barber)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestBarberBookingAlertIsGetOnly(t *testing.T) {
	barber, _, _ := alertFixture(t)

	response := httptest.NewRecorder()
	BarberNotificationsHandler(response, httptest.NewRequest(http.MethodPost, "/barber/notifications", nil), barber)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
}

// The alert read must stay cheap: a second barber's bookings never enter the
// query, and the baseline id tracks the newest request for this barber only.
func TestBarberBookingAlertUsesContext(t *testing.T) {
	barber, customer, style := alertFixture(t)
	newAlertBooking(t, barber, customer, style, "09:00")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := storage.PendingBookingsForBarberAfter(ctx, barber.ID, 0); err == nil {
		t.Fatal("expected a cancelled context to abort the read")
	}
}