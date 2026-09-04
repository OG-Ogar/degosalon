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

	Text   string
	SentAt time.Time
}

// FromBarber reports whether the barber wrote this message.
func (m Message) FromBarber() bool {
	return m.SenderRole == SenderBarber
}
