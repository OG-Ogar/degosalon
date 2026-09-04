package handlers

import (
	"strconv"
	"strings"
)

// pathParts splits a request path into its non-empty segments, so
// "/barber/hairstyles/5/edit" becomes
// ["barber", "hairstyles", "5", "edit"].
//
// Routing on segments replaces the previous strings.HasSuffix matching,
// which routed any path ending in "/edit" to the hairstyle editor no
// matter what came before it.
func pathParts(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "/")
}

// parseID reads a positive integer path segment.
func parseID(segment string) (int, bool) {
	id, err := strconv.Atoi(segment)
	if err != nil || id < 1 {
		return 0, false
	}

	return id, true
}

// trimmed reads a form field with surrounding whitespace removed.
func trimmedField(value string) string {
	return strings.TrimSpace(value)
}

// normalisedEmail lowercases and trims an email so that lookups and the
// uniqueness checks agree with each other.
func normalisedEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// formatPrice renders a price the way the form expects to read it back.
func formatPrice(price float64) string {
	return strconv.FormatFloat(price, 'f', 2, 64)
}

// formatInt renders a whole number for a form field.
func formatInt(value int) string {
	return strconv.Itoa(value)
}

// hairstylePath builds /barber/hairstyles/{id}/{action}.
func hairstylePath(id int, action string) string {
	return "/barber/hairstyles/" + strconv.Itoa(id) + "/" + action
}

// bookingActionPath builds the URL a booking status form posts to.
func bookingActionPath(prefix string, id int, action string) string {
	return prefix + strconv.Itoa(id) + "/" + action
}
