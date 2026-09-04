package handlers

import (
	"net/http"
	"strconv"
	"time"
)

// readMessage validates a chat submission. An empty message is rejected
// rather than stored, and an over-long one is trimmed to the cap instead
// of being thrown away.
func readMessage(r *http.Request) (string, string) {
	text := trimmedField(r.PostFormValue("text"))

	if text == "" {
		return "", "Type a message before sending."
	}

	if len(text) > maxMessageLength {
		text = text[:maxMessageLength]
	}

	return text, ""
}

// chatPath builds the URL of one conversation thread.
func chatPath(prefix string, id int) string {
	return prefix + strconv.Itoa(id)
}

// formatWhen renders a timestamp for a conversation list. Times today
// show only the clock, older ones show the date, which is what makes a
// list of threads scannable.
func formatWhen(at time.Time) string {
	if at.IsZero() {
		return ""
	}

	now := time.Now()

	if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
		return at.Format("15:04")
	}

	if at.Year() == now.Year() {
		return at.Format("2 Jan, 15:04")
	}

	return at.Format("2 Jan 2006")
}
