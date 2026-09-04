package storage

import (
	"sort"
	"time"

	"barber-booking/models"
)

// Conversation summarises the chat between one barber and one customer.
// It is a query result, not a stored record: the messages themselves
// carry the pairing.
type Conversation struct {
	BarberID   int
	CustomerID int

	LastText string
	LastAt   time.Time
	Count    int
}

// AddMessage assigns an ID and appends the message to its conversation.
func AddMessage(message models.Message) (models.Message, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, found := findBarberByID(message.BarberID); !found {
		return models.Message{}, ErrNotFound
	}

	if _, found := findCustomerByID(message.CustomerID); !found {
		return models.Message{}, ErrNotFound
	}

	message.ID = nextID(&lastMessageID)

	if message.SentAt.IsZero() {
		message.SentAt = time.Now()
	}

	messages = append(messages, message)

	return message, nil
}

// ListConversation returns the messages between one barber and one
// customer, oldest first.
func ListConversation(barberID, customerID int) []models.Message {
	mu.RLock()
	defer mu.RUnlock()

	var thread []models.Message

	for _, message := range messages {
		if message.BarberID == barberID &&
			message.CustomerID == customerID {

			thread = append(thread, message)
		}
	}

	sort.SliceStable(thread, func(i, j int) bool {
		return thread[i].ID < thread[j].ID
	})

	return thread
}

// ConversationsForBarber lists every customer this barber has chatted
// with, most recently active first.
func ConversationsForBarber(barberID int) []Conversation {
	return summarise(func(m models.Message) bool {
		return m.BarberID == barberID
	})
}

// ConversationsForCustomer lists every barber this customer has chatted
// with, most recently active first.
func ConversationsForCustomer(customerID int) []Conversation {
	return summarise(func(m models.Message) bool {
		return m.CustomerID == customerID
	})
}

// summarise groups the matching messages into one Conversation per
// barber/customer pair.
func summarise(match func(models.Message) bool) []Conversation {
	mu.RLock()
	defer mu.RUnlock()

	type key struct {
		barberID   int
		customerID int
	}

	index := map[key]*Conversation{}

	for _, message := range messages {
		if !match(message) {
			continue
		}

		id := key{message.BarberID, message.CustomerID}

		conversation, seen := index[id]
		if !seen {
			conversation = &Conversation{
				BarberID:   message.BarberID,
				CustomerID: message.CustomerID,
			}

			index[id] = conversation
		}

		conversation.Count++

		if !message.SentAt.Before(conversation.LastAt) {
			conversation.LastAt = message.SentAt
			conversation.LastText = message.Text
		}
	}

	list := make([]Conversation, 0, len(index))

	for _, conversation := range index {
		list = append(list, *conversation)
	}

	sort.SliceStable(list, func(i, j int) bool {
		return list[i].LastAt.After(list[j].LastAt)
	})

	return list
}
