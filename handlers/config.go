package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"barber-booking/security"
)

// The application has exactly one administrator. Their credentials come
// from the environment instead of being written into the source, and the
// password is stored only as a hash.
var (
	adminUsername     = "admin"
	adminPasswordHash string
)

// InitAdmin prepares the administrator account.
//
// ADMIN_USERNAME and ADMIN_PASSWORD override the defaults. When no
// password is configured a random one is generated and returned so that
// main can print it once at startup; that keeps a fresh checkout usable
// without shipping a well-known password like the previous hardcoded
// admin/password123.
func InitAdmin() (generated string, err error) {
	if name := strings.TrimSpace(os.Getenv("ADMIN_USERNAME")); name != "" {
		adminUsername = name
	}

	password := os.Getenv("ADMIN_PASSWORD")

	if password == "" {
		buf := make([]byte, 12)

		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("generate admin password: %w", err)
		}

		password = base64.RawURLEncoding.EncodeToString(buf)
		generated = password
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return "", err
	}

	adminPasswordHash = hash

	return generated, nil
}

// AdminUsername reports the configured administrator name, for the
// startup banner.
func AdminUsername() string {
	return adminUsername
}

// InitBookingHours applies BOOKING_OPEN and BOOKING_CLOSE, each a whole
// hour of the day. Bookings are taken from the first to the second
// inclusive, so a shop that works until 22:00 sets BOOKING_CLOSE=22
// rather than editing the source.
func InitBookingHours() error {
	zone := strings.TrimSpace(os.Getenv("BOOKING_TIMEZONE"))
	if zone == "" { zone = "Africa/Lagos" }
	location, err := time.LoadLocation(zone)
	if err != nil { return fmt.Errorf("BOOKING_TIMEZONE %q is invalid: %w", zone, err) }
	bookingLocation = location

	from, err := hourFromEnv("BOOKING_OPEN", openingHour)
	if err != nil {
		return err
	}

	until, err := hourFromEnv("BOOKING_CLOSE", closingHour)
	if err != nil {
		return err
	}

	if from >= until {
		return fmt.Errorf(
			"BOOKING_OPEN %s is not before BOOKING_CLOSE %s",
			formatHour(from),
			formatHour(until),
		)
	}

	openingHour, closingHour = from, until

	return nil
}

// BookingHours reports the bookable window, for the startup banner.
func BookingHours() (string, string) {
	return formatHour(openingHour), formatHour(closingHour)
}

// hourFromEnv reads one hour of the day, keeping the fallback when the
// variable is unset.
func hourFromEnv(name string, fallback int) (int, error) {
	text := strings.TrimSpace(os.Getenv(name))

	if text == "" {
		return fallback, nil
	}

	hour, err := strconv.Atoi(text)
	if err != nil || hour < 0 || hour > 24 {
		return 0, fmt.Errorf("%s must be an hour from 0 to 24, not %q", name, text)
	}

	return hour, nil
}

// validAdminCredentials checks a login attempt. Both comparisons run
// before the result is combined so that a wrong username costs the same
// as a wrong password.
func validAdminCredentials(username, password string) bool {
	nameMatches := subtle.ConstantTimeCompare(
		[]byte(username),
		[]byte(adminUsername),
	) == 1

	passwordMatches := security.VerifyPassword(adminPasswordHash, password)

	return nameMatches && passwordMatches
}
