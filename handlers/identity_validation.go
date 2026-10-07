package handlers

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Validate submitted text without silently truncating it. Names and locations
// accept international scripts and punctuation, but not control characters.
func validIdentityText(label, value string, limit int) string {
	if value == "" {
		return label + " is required."
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return fmt.Sprintf("%s must be valid text of at most %d characters.", label, limit)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return label + " must not contain control characters."
		}
	}
	return ""
}

func validateIdentity(name, email, phone string) string {
	if problem := validIdentityText("Full name", name, 120); problem != "" {
		return problem
	}
	if problem := validIdentityText("Email", email, 254); problem != "" {
		return problem
	}
	address, err := mail.ParseAddress(email)
	local, domain, hasAt := strings.Cut(email, "@")
	if err != nil || address.Address != email || !hasAt || len(email) > 254 || len(local) > 64 ||
		!strings.Contains(domain, ".") || strings.ContainsAny(email, " \t\r\n") {
		return "Enter a valid email address."
	}
	if problem := validIdentityText("Phone number", phone, 32); problem != "" {
		return problem
	}
	digits := 0
	for index, character := range phone {
		switch {
		case character >= '0' && character <= '9':
			digits++
		case character == '+' && index == 0:
		case character == ' ' || character == '-' || character == '(' || character == ')':
		default:
			return "Enter a phone number using digits and optional leading +, spaces, hyphens or parentheses."
		}
	}
	if digits < 7 || digits > 15 {
		return "Phone number must contain between 7 and 15 digits."
	}
	return ""
}
