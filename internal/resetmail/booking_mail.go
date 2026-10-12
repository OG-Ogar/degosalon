package resetmail

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// BookingNotification contains appointment details sent to a barber
// when a customer books a haircut appointment.
type BookingNotification struct {
	BarberEmail     string
	BarberName      string
	ShopName        string
	ShopAddress     string
	CustomerName    string
	CustomerPhone   string
	CustomerEmail   string
	ServiceName     string
	ServicePrice    float64
	ServiceDuration int
	Date            string
	Time            string
	Notes           string
	BookingID       int
}

// FormatNaira formats a decimal currency amount according to NGN formatting rules (e.g. ₦1,500.00).
func FormatNaira(amount float64) string {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return "₦0.00"
	}
	text := fmt.Sprintf("%.2f", amount)
	parts := strings.SplitN(text, ".", 2)
	digits := parts[0]
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign = "-"
		digits = strings.TrimPrefix(digits, "-")
	}
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return "₦" + sign + digits + "." + parts[1]
}

// FormatDuration formats appointment duration in minutes (e.g. "30 minutes" or "1 minute").
func FormatDuration(minutes int) string {
	if minutes == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", minutes)
}

// BarberBookingsLink returns the canonical action link for a barber to review appointments.
func (c Config) BarberBookingsLink() (string, error) {
	if c.origin == "" {
		return "", ErrConfiguration
	}
	return c.origin + "/barber/bookings", nil
}

// BookingEmailContent renders the subject and plain text message for a booking notification.
func BookingEmailContent(origin string, n BookingNotification) (string, string) {
	customerName := strings.TrimSpace(n.CustomerName)
	if customerName == "" {
		customerName = "Customer"
	}
	serviceName := strings.TrimSpace(n.ServiceName)
	if serviceName == "" {
		serviceName = "Haircut"
	}

	subject := fmt.Sprintf("New Haircut Booking Request: %s - %s", customerName, serviceName)
	subject = strings.ReplaceAll(subject, "\r", "")
	subject = strings.ReplaceAll(subject, "\n", "")

	notes := strings.TrimSpace(n.Notes)
	if notes == "" {
		notes = "None"
	}

	customerPhone := strings.TrimSpace(n.CustomerPhone)
	if customerPhone == "" {
		customerPhone = "Not provided"
	}

	customerEmail := strings.TrimSpace(n.CustomerEmail)
	if customerEmail == "" {
		customerEmail = "Not provided"
	}

	greeting := "Hello,"
	if barberName := strings.TrimSpace(n.BarberName); barberName != "" {
		greeting = fmt.Sprintf("Hello %s,", barberName)
	}

	shopInfo := strings.TrimSpace(n.ShopName)
	if addr := strings.TrimSpace(n.ShopAddress); addr != "" {
		if shopInfo != "" {
			shopInfo += ", " + addr
		} else {
			shopInfo = addr
		}
	}
	if shopInfo == "" {
		shopInfo = "De Go Salon"
	}

	dateStr := strings.TrimSpace(n.Date)
	if dateStr == "" {
		dateStr = "Unspecified"
	}
	timeStr := strings.TrimSpace(n.Time)
	if timeStr == "" {
		timeStr = "Unspecified"
	}

	link := origin + "/barber/bookings"

	body := fmt.Sprintf(`%s

You have received a new haircut booking request on De Go Salon!

Booking Details:
- Customer Name: %s
- Customer Phone: %s
- Customer Email: %s
- Service: %s
- Price: %s
- Duration: %s
- Appointment Date: %s
- Appointment Time: %s
- Shop: %s
- Customer Notes: %s

Please review, accept, or reject this request:
%s
`,
		greeting,
		customerName,
		customerPhone,
		customerEmail,
		serviceName,
		FormatNaira(n.ServicePrice),
		FormatDuration(n.ServiceDuration),
		dateStr,
		timeStr,
		shopInfo,
		notes,
		link,
	)

	return subject, body
}

// SendBookingNotification dispatches an appointment alert to the booked barber.
// It supports both Brevo API and authenticated TLS/STARTTLS SMTP transports.
func (c Config) SendBookingNotification(ctx context.Context, n BookingNotification) error {
	if !validAddress(n.BarberEmail) || c.origin == "" {
		return ErrConfiguration
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	subject, body := BookingEmailContent(c.origin, n)

	if c.apiKey != "" {
		return c.sendBrevoMessage(ctx, n.BarberEmail, subject, body)
	}

	return c.sendSMTP(ctx, n.BarberEmail, subject, body)
}

