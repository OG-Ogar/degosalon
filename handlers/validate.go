package handlers

import (
	"fmt"
	"strings"
	"time"
)

const (
	// minPasswordLength is the shortest password accepted anywhere in
	// the application.
	minPasswordLength = 8

	// maxTextLength caps free text so one request cannot store an
	// unbounded amount of data in memory.
	maxTextLength = 2000

	// openingHour and closingHour bound the bookable part of the day.
	openingHour = 8
	closingHour = 20
)

// passwordTooShortMessage is shown when a new password is rejected.
var passwordTooShortMessage = fmt.Sprintf(
	"Password must be at least %d characters long.",
	minPasswordLength,
)

// truncateText trims a field and caps its length.
func truncateText(value string) string {
	value = strings.TrimSpace(value)

	if len(value) > maxTextLength {
		return value[:maxTextLength]
	}

	return value
}

// validBookingDate accepts a YYYY-MM-DD date that is not in the past and
// returns it normalised, or a message explaining the rejection.
func validBookingDate(value string) (string, string) {
	day, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return "", "Choose a valid date in YYYY-MM-DD form."
	}

	normalised := day.Format("2006-01-02")

	// ISO dates sort the same as they compare, so string comparison is
	// enough here and avoids timezone arithmetic.
	if normalised < time.Now().Format("2006-01-02") {
		return "", "That date has already passed."
	}

	return normalised, ""
}

// validBookingTime accepts an HH:MM time inside opening hours.
func validBookingTime(value string) (string, string) {
	at, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return "", "Choose a valid time in HH:MM form."
	}

	if at.Hour() < openingHour || at.Hour() >= closingHour {
		return "", fmt.Sprintf(
			"Bookings are only taken between %02d:00 and %02d:00.",
			openingHour,
			closingHour,
		)
	}

	return at.Format("15:04"), ""
}

// today is the earliest date a booking form will accept, used as the min
// attribute on the date input.
func today() string {
	return time.Now().Format("2006-01-02")
}

// formatHour renders an opening-hours bound as HH:MM.
func formatHour(hour int) string {
	return fmt.Sprintf("%02d:00", hour)
}
