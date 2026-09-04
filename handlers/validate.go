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
)

// openingHour and closingHour bound the bookable part of the day. They
// are the defaults for a shop that works 08:00 to 20:00; InitBookingHours
// replaces them from the environment at startup, and both the booking
// form and the check below read them, so the two can never disagree.
var (
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
//
// The closing time itself is the last slot that can be asked for, which
// is what the form's max attribute already offers the customer. Rejecting
// it here as well used to make 20:00 the one time the browser accepted and
// the server refused.
func validBookingTime(value string) (string, string) {
	at, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return "", "Choose a valid time in HH:MM form."
	}

	minute := at.Hour()*60 + at.Minute()

	if minute < openingHour*60 || minute > closingHour*60 {
		return "", fmt.Sprintf(
			"Bookings are only taken between %s and %s.",
			formatHour(openingHour),
			formatHour(closingHour),
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
