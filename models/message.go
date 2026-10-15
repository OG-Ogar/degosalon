package models

import "time"

// Sender roles for a chat message.
const (
	SenderBarber   = "barber"
	SenderCustomer = "customer"
)

// Message is a single chat message inside the conversation between one
// barber and one customer. A conversation is identified by the
// (BarberID, CustomerID) pair, so no separate conversation record is
// needed.
type Message struct {
	ID int

	BarberID   int
	CustomerID int

	// SenderRole is SenderBarber or SenderCustomer and decides which
	// side of the chat the message is drawn on.
	SenderRole string
	SenderName string

	Text string

	// Image and Audio hold the stored file names of an attached photo or
	// voice note. Both are empty for a plain text message.
	Image string
	Audio string

	SentAt time.Time
}

// FromBarber reports whether the barber wrote this message.
func (m Message) FromBarber() bool {
	return m.SenderRole == SenderBarber
}

// HasImage reports whether the message carries a photo.
func (m Message) HasImage() bool {
	return m.Image != ""
}

// HasAudio reports whether the message carries a voice note.
func (m Message) HasAudio() bool {
	return m.Audio != ""
}
