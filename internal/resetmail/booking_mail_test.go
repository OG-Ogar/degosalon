package resetmail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/quotedprintable"
	"net/http"
	"strings"
	"testing"
	"time"
)

func sampleBookingNotification() BookingNotification {
	return BookingNotification{
		BarberEmail:     "barber@example.com",
		BarberName:      "Dele Barber",
		ShopName:        "Dele's Cuts",
		ShopAddress:     "12 Commercial Ave, Yaba, Lagos",
		CustomerName:    "Tunde Adebayo",
		CustomerPhone:   "08012345678",
		CustomerEmail:   "tunde@example.com",
		ServiceName:     "Classic Low Fade",
		ServicePrice:    15000.0,
		ServiceDuration: 45,
		Date:            "2026-10-25",
		Time:            "14:00",
		Notes:           "Please make the side taper sharp.",
		BookingID:       42,
	}
}

func TestFormatNaira(t *testing.T) {
	for _, tc := range []struct {
		amount float64
		want   string
	}{
		{0, "₦0.00"},
		{1500, "₦1,500.00"},
		{15000, "₦15,000.00"},
		{150000.5, "₦150,000.50"},
		{9999999.99, "₦9,999,999.99"},
	} {
		if got := FormatNaira(tc.amount); got != tc.want {
			t.Errorf("FormatNaira(%v) = %q, want %q", tc.amount, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	if got := FormatDuration(1); got != "1 minute" {
		t.Fatalf("FormatDuration(1) = %q, want '1 minute'", got)
	}
	if got := FormatDuration(45); got != "45 minutes" {
		t.Fatalf("FormatDuration(45) = %q, want '45 minutes'", got)
	}
}

func TestBookingEmailContent(t *testing.T) {
	origin := "https://salon.example.com"
	n := sampleBookingNotification()

	subject, body := BookingEmailContent(origin, n)

	wantSubject := "New Haircut Booking Request: Tunde Adebayo - Classic Low Fade"
	if subject != wantSubject {
		t.Fatalf("subject = %q, want %q", subject, wantSubject)
	}

	for _, want := range []string{
		"Hello Dele Barber,",
		"Tunde Adebayo",
		"08012345678",
		"tunde@example.com",
		"Classic Low Fade",
		"₦15,000.00",
		"45 minutes",
		"2026-10-25",
		"14:00",
		"Dele's Cuts, 12 Commercial Ave, Yaba, Lagos",
		"Please make the side taper sharp.",
		"https://salon.example.com/barber/bookings",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

func TestBookingEmailContentDefaults(t *testing.T) {
	origin := "https://salon.example.com"
	n := BookingNotification{
		BarberEmail: "barber@example.com",
	}

	subject, body := BookingEmailContent(origin, n)

	if subject != "New Haircut Booking Request: Customer - Haircut" {
		t.Fatalf("default subject = %q", subject)
	}

	for _, want := range []string{
		"Hello,",
		"Customer Name: Customer",
		"Phone: Not provided",
		"Email: Not provided",
		"Service: Haircut",
		"Price: ₦0.00",
		"Duration: 0 minutes",
		"Appointment Date: Unspecified",
		"Appointment Time: Unspecified",
		"Shop: De Go Salon",
		"Customer Notes: None",
		"https://salon.example.com/barber/bookings",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing default %q:\n%s", want, body)
		}
	}
}

func TestSendBookingNotificationValidation(t *testing.T) {
	c := Config{origin: "https://salon.example.com"}

	// Invalid email
	err := c.SendBookingNotification(context.Background(), BookingNotification{BarberEmail: "invalid-email"})
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected ErrConfiguration for invalid email, got %v", err)
	}

	// Missing origin
	emptyConfig := Config{}
	err = emptyConfig.SendBookingNotification(context.Background(), BookingNotification{BarberEmail: "barber@example.com"})
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected ErrConfiguration for missing origin, got %v", err)
	}
}

func TestSendBookingNotificationBrevo(t *testing.T) {
	c := brevoConfig(t)
	n := sampleBookingNotification()

	var capturedMsg brevoMessage
	c.httpClient = &http.Client{Transport: brevoRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.String() != brevoEndpoint ||
			r.Header.Get("api-key") != c.apiKey || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("incorrect authenticated API request")
		}
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 10*time.Second {
			t.Fatal("delivery must have bounded deadline")
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedMsg); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"messageId":"accepted-booking-notification"}`)),
		}, nil
	})}

	err := c.SendBookingNotification(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedMsg.Sender.Email != c.from {
		t.Fatalf("sender = %q, want %q", capturedMsg.Sender.Email, c.from)
	}
	if len(capturedMsg.To) != 1 || capturedMsg.To[0].Email != n.BarberEmail {
		t.Fatalf("recipient = %+v, want %q", capturedMsg.To, n.BarberEmail)
	}
	if capturedMsg.Subject != "New Haircut Booking Request: Tunde Adebayo - Classic Low Fade" {
		t.Fatalf("subject = %q", capturedMsg.Subject)
	}
	for _, want := range []string{
		"Tunde Adebayo",
		"08012345678",
		"Classic Low Fade",
		"₦15,000.00",
		"45 minutes",
		"2026-10-25",
		"14:00",
		"https://salon.example.com/barber/bookings",
	} {
		if !strings.Contains(capturedMsg.TextContent, want) {
			t.Fatalf("Brevo textContent missing %q:\n%s", want, capturedMsg.TextContent)
		}
	}
}

func TestSendBookingNotificationSMTP(t *testing.T) {
	c, message := relay(t, false)
	n := sampleBookingNotification()

	err := c.SendBookingNotification(context.Background(), n)
	if err != nil {
		t.Fatalf("SMTP send failed: %v", err)
	}

	wire := <-message
	parts := strings.SplitN(wire, "\n\n", 2)
	if len(parts) != 2 {
		t.Fatalf("missing MIME body: %q", wire)
	}

	if !strings.Contains(wire, "To: barber@example.com") {
		t.Fatalf("missing To header in wire: %s", wire)
	}
	if !strings.Contains(wire, "Subject: New Haircut Booking Request: Tunde Adebayo - Classic Low Fade") {
		t.Fatalf("missing Subject header in wire: %s", wire)
	}

	body, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(parts[1])))
	if err != nil {
		t.Fatal(err)
	}
	bodyStr := string(body)

	for _, want := range []string{
		"Tunde Adebayo",
		"08012345678",
		"Classic Low Fade",
		"₦15,000.00",
		"45 minutes",
		"2026-10-25",
		"14:00",
		"https://salon.example.com/barber/bookings",
	} {
		if !strings.Contains(bodyStr, want) {
			t.Fatalf("SMTP body missing %q:\n%s", want, bodyStr)
		}
	}
}

