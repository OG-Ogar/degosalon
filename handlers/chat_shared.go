package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"
)

// readMessage validates a text-only chat submission. An empty message is
// rejected rather than stored, and an over-long one is trimmed to the cap
// instead of being thrown away.
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

// readChatMessage validates a submission that may carry text, a photo
// (field "photo") or a voice note (field "voice"). Text is optional when
// an attachment is present. A message with none of the three is rejected.
func readChatMessage(r *http.Request) (text, image, audio, problem string) {
	text = trimmedField(r.PostFormValue("text"))

	if len(text) > maxMessageLength {
		text = text[:maxMessageLength]
	}

	var err error

	image, err = chatImages.save(r, "photo")
	if err != nil {
		if !errors.Is(err, errNoUpload) {
			// The real reason goes to the server log, never to the page.
			log.Printf("chat photo upload failed: %v", err)

			return text, "", "", "The photo could not be sent. Use a JPEG, PNG or WebP image under 5 MB."
		}

		image = ""
	}

	audio, err = chatVoice.save(r, "voice")
	if err != nil {
		if !errors.Is(err, errNoUpload) {
			log.Printf("chat voice upload failed: %v", err)
			removeChatMedia(image)

			return text, "", "", "The voice note could not be sent. Keep it under 2 minutes and try again."
		}

		audio = ""
	}

	if text == "" && image == "" && audio == "" {
		return "", "", "", "Type a message, add a photo or record a voice note."
	}

	return text, image, audio, ""
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