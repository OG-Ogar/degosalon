package storage

import (
	"context"
	"database/sql"
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
func AddMessage(ctx context.Context, message models.Message) (models.Message, error) {
	if err := ctx.Err(); err != nil {
		return models.Message{}, err
	}
	if databaseEnabled() {
		if message.SentAt.IsZero() {
			message.SentAt = time.Now()
		}
		err := db.QueryRowContext(ctx, "insert into public.messages(barber_id,customer_id,sender_role,sender_name,text,sent_at) values($1,$2,$3,$4,$5,$6) returning id", message.BarberID, message.CustomerID, message.SenderRole, message.SenderName, message.Text, message.SentAt).Scan(&message.ID)
		return message, err
	}
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
func ListConversation(ctx context.Context, barberID, customerID int) ([]models.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		return queryRecords(ctx, "select id,barber_id,customer_id,sender_role,sender_name,text,sent_at from public.messages where barber_id=$1 and customer_id=$2 order by id", func(rows *sql.Rows) (models.Message, error) {
			var v models.Message
			err := rows.Scan(&v.ID, &v.BarberID, &v.CustomerID, &v.SenderRole, &v.SenderName, &v.Text, &v.SentAt)
			return v, err
		}, barberID, customerID)
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]models.Message, 0)
	for _, v := range messages {
		if v.BarberID == barberID && v.CustomerID == customerID {
			out = append(out, v)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ConversationsForBarber lists every customer this barber has chatted
// with, most recently active first.
func ConversationsForBarber(ctx context.Context, id int) ([]Conversation, error) {
	return scopedConversations(ctx, "barber_id", id)
}

// ConversationsForCustomer lists every barber this customer has chatted
// with, most recently active first.
func ConversationsForCustomer(ctx context.Context, id int) ([]Conversation, error) {
	return scopedConversations(ctx, "customer_id", id)
}

// summarise groups the matching messages into one Conversation per
// barber/customer pair.
func scopedConversations(ctx context.Context, field string, id int) ([]Conversation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		query := `select barber_id,customer_id,text,sent_at,message_count from (
   select distinct on (m.barber_id,m.customer_id) m.barber_id,m.customer_id,m.text,m.sent_at,
   count(*) over(partition by m.barber_id,m.customer_id) as message_count
   from public.messages m where m.` + field + `=$1 order by m.barber_id,m.customer_id,m.sent_at desc,m.id desc
  ) latest order by sent_at desc,barber_id,customer_id`
		return queryRecords(ctx, query, func(rows *sql.Rows) (Conversation, error) {
			var v Conversation
			err := rows.Scan(&v.BarberID, &v.CustomerID, &v.LastText, &v.LastAt, &v.Count)
			return v, err
		}, id)
	}
	mu.RLock()
	defer mu.RUnlock()
	return summariseMessages(messages, func(v models.Message) bool {
		if field == "barber_id" {
			return v.BarberID == id
		}
		return v.CustomerID == id
	}), nil
}

func summariseMessages(source []models.Message, match func(models.Message) bool) []Conversation {
	type key struct{ barberID, customerID int }
	index := map[key]*Conversation{}
	for _, message := range source {
		if !match(message) {
			continue
		}
		id := key{message.BarberID, message.CustomerID}
		conversation := index[id]
		if conversation == nil {
			conversation = &Conversation{BarberID: message.BarberID, CustomerID: message.CustomerID}
			index[id] = conversation
		}
		conversation.Count++
		if !message.SentAt.Before(conversation.LastAt) {
			conversation.LastAt = message.SentAt
			conversation.LastText = message.Text
		}
	}
	list := make([]Conversation, 0, len(index))
	for _, v := range index {
		list = append(list, *v)
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].LastAt.After(list[j].LastAt) })
	return list
}
