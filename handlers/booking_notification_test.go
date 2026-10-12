package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"barber-booking/internal/resetmail"
	"barber-booking/models"
	"barber-booking/storage"
)

func TestBookingEmailNotificationDispatchedOnBooking(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	barber, err := storage.CreateBarber(models.Barber{
		Email:       "dele_barber@example.com",
		FullName:    "Dele Barber",
		ShopName:    "Dele's Cuts",
		ShopAddress: "12 Commercial Ave, Yaba, Lagos",
		Status:      models.BarberActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	customer, err := storage.CreateCustomer(models.Customer{
		Email:       "tunde_customer@example.com",
		FullName:    "Tunde Adebayo",
		PhoneNumber: "08012345678",
	})
	if err != nil {
		t.Fatal(err)
	}

	style, err := storage.CreateHairstyle(models.Hairstyle{
		BarberID:    barber.ID,
		Name:        "Classic Low Fade",
		Price:       7500,
		Duration:    30,
		IsAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	origSender := bookingMailSender
	origHook := onBookingNotificationDispatched
	t.Cleanup(func() {
		bookingNotificationWG.Wait()
		bookingMailSender = origSender
		onBookingNotificationDispatched = origHook
	})

	dispatched := make(chan resetmail.BookingNotification, 1)
	bookingMailSender = func(ctx context.Context, n resetmail.BookingNotification) error {
		dispatched <- n
		return nil
	}

	bookingDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	form := url.Values{
		"csrf_token":   {"test-csrf-token"},
		"barber_id":    {strconv.Itoa(barber.ID)},
		"hairstyle_id": {strconv.Itoa(style.ID)},
		"date":         {bookingDate},
		"time":         {"10:00"},
		"notes":        {"Please make it neat."},
	}

	req := httptest.NewRequest(http.MethodPost, "/customer/book", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()

	BookHairstyleHandler(res, req, customer, sessionData{CSRFToken: "test-csrf-token"})

	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303 See Other, got %d. Body: %s", res.Code, res.Body.String())
	}
	loc := res.Header().Get("Location")
	if !strings.HasPrefix(loc, "/customer/booking-confirmation/") {
		t.Fatalf("expected redirect to /customer/booking-confirmation/{id}, got %q", loc)
	}

	select {
	case n := <-dispatched:
		if n.BarberEmail != barber.Email {
			t.Errorf("BarberEmail = %q, want %q", n.BarberEmail, barber.Email)
		}
		if n.BarberName != barber.FullName {
			t.Errorf("BarberName = %q, want %q", n.BarberName, barber.FullName)
		}
		if n.CustomerName != customer.FullName {
			t.Errorf("CustomerName = %q, want %q", n.CustomerName, customer.FullName)
		}
		if n.CustomerPhone != customer.PhoneNumber {
			t.Errorf("CustomerPhone = %q, want %q", n.CustomerPhone, customer.PhoneNumber)
		}
		if n.CustomerEmail != customer.Email {
			t.Errorf("CustomerEmail = %q, want %q", n.CustomerEmail, customer.Email)
		}
		if n.ServiceName != style.Name {
			t.Errorf("ServiceName = %q, want %q", n.ServiceName, style.Name)
		}
		if n.ServicePrice != style.Price {
			t.Errorf("ServicePrice = %v, want %v", n.ServicePrice, style.Price)
		}
		if n.ServiceDuration != style.Duration {
			t.Errorf("ServiceDuration = %d, want %d", n.ServiceDuration, style.Duration)
		}
		if n.Date != bookingDate {
			t.Errorf("Date = %q, want %q", n.Date, bookingDate)
		}
		if n.Time != "10:00" {
			t.Errorf("Time = %q, want '10:00'", n.Time)
		}
		if n.Notes != "Please make it neat." {
			t.Errorf("Notes = %q, want 'Please make it neat.'", n.Notes)
		}
		if n.BookingID <= 0 {
			t.Errorf("BookingID = %d, want > 0", n.BookingID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for booking notification dispatch")
	}
}

func TestBookingSucceedsEvenIfEmailFailsOrUnconfigured(t *testing.T) {
	origSender := bookingMailSender
	origHook := onBookingNotificationDispatched
	t.Cleanup(func() {
		bookingNotificationWG.Wait()
		bookingMailSender = origSender
		onBookingNotificationDispatched = origHook
	})

	for idx, tc := range []struct {
		name string
		err  error
		time string
	}{
		{"mail delivery failure", errors.New("smtp connection failed"), "11:00"},
		{"mail unconfigured", resetmail.ErrConfiguration, "11:30"},
	} {
		_ = idx
		t.Run(tc.name, func(t *testing.T) {
			storage.Reset()
			t.Cleanup(storage.Reset)

			barber, err := storage.CreateBarber(models.Barber{
				Email:       "barber_fail@example.com",
				FullName:    "Barber Fail",
				ShopName:    "Fail Safe Salon",
				ShopAddress: "1 Fail St, Lagos",
				Status:      models.BarberActive,
			})
			if err != nil {
				t.Fatal(err)
			}

			customer, err := storage.CreateCustomer(models.Customer{
				Email:       "buyer@example.com",
				FullName:    "Buyer Name",
				PhoneNumber: "08099998888",
			})
			if err != nil {
				t.Fatal(err)
			}

			style, err := storage.CreateHairstyle(models.Hairstyle{
				BarberID:    barber.ID,
				Name:        "Buzz Cut",
				Price:       3000,
				Duration:    20,
				IsAvailable: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			dispatchedHook := make(chan error, 1)
			bookingMailSender = func(ctx context.Context, n resetmail.BookingNotification) error {
				return tc.err
			}
			onBookingNotificationDispatched = func(b models.Booking, ba models.Barber, err error) {
				dispatchedHook <- err
			}

			bookingDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
			form := url.Values{
				"csrf_token":   {"csrf-token"},
				"barber_id":    {strconv.Itoa(barber.ID)},
				"hairstyle_id": {strconv.Itoa(style.ID)},
				"date":         {bookingDate},
				"time":         {tc.time},
				"notes":        {""},
			}

			req := httptest.NewRequest(http.MethodPost, "/customer/book", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			res := httptest.NewRecorder()

			BookHairstyleHandler(res, req, customer, sessionData{CSRFToken: "csrf-token"})

			if res.Code != http.StatusSeeOther {
				t.Fatalf("expected redirect despite mail failure, got %d: %s", res.Code, res.Body.String())
			}

			select {
			case gotErr := <-dispatchedHook:
				if !errors.Is(gotErr, tc.err) {
					t.Fatalf("hook got err = %v, want %v", gotErr, tc.err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("timed out waiting for hook")
			}
			bookingNotificationWG.Wait()
		})
	}
}

func TestBookingSkipsEmailIfBarberEmailEmpty(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	barber, err := storage.CreateBarber(models.Barber{
		Email:       "",
		FullName:    "No Email Barber",
		ShopName:    "No Email Shop",
		ShopAddress: "No Email Address",
		Status:      models.BarberActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	customer, err := storage.CreateCustomer(models.Customer{
		Email:       "buyer@example.com",
		FullName:    "Buyer Name",
		PhoneNumber: "08099998888",
	})
	if err != nil {
		t.Fatal(err)
	}

	style, err := storage.CreateHairstyle(models.Hairstyle{
		BarberID:    barber.ID,
		Name:        "Buzz Cut",
		Price:       3000,
		Duration:    20,
		IsAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	origSender := bookingMailSender
	origHook := onBookingNotificationDispatched
	t.Cleanup(func() {
		bookingNotificationWG.Wait()
		bookingMailSender = origSender
		onBookingNotificationDispatched = origHook
	})

	called := false
	bookingMailSender = func(ctx context.Context, n resetmail.BookingNotification) error {
		called = true
		return nil
	}

	bookingDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	form := url.Values{
		"csrf_token":   {"csrf-token"},
		"barber_id":    {strconv.Itoa(barber.ID)},
		"hairstyle_id": {strconv.Itoa(style.ID)},
		"date":         {bookingDate},
		"time":         {"12:00"},
	}

	req := httptest.NewRequest(http.MethodPost, "/customer/book", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()

	BookHairstyleHandler(res, req, customer, sessionData{CSRFToken: "csrf-token"})

	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d: %s", res.Code, res.Body.String())
	}
	if called {
		t.Fatal("sender should not be called when barber email is empty")
	}
}
