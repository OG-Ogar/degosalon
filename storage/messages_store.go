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
	if databaseEnabled(){if message.SentAt.IsZero(){message.SentAt=time.Now()};err:=db.QueryRow("insert into public.messages(barber_id,customer_id,sender_role,sender_name,text,sent_at) values($1,$2,$3,$4,$5,$6) returning id",message.BarberID,message.CustomerID,message.SenderRole,message.SenderName,message.Text,message.SentAt).Scan(&message.ID);return message,err}
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
	if databaseEnabled(){rows,err:=db.Query("select id,barber_id,customer_id,sender_role,sender_name,text,sent_at from public.messages where barber_id=$1 and customer_id=$2 order by id",barberID,customerID);if err==nil{defer rows.Close();var out []models.Message;for rows.Next(){var v models.Message;if rows.Scan(&v.ID,&v.BarberID,&v.CustomerID,&v.SenderRole,&v.SenderName,&v.Text,&v.SentAt)==nil{out=append(out,v)}};return out}}
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
	if databaseEnabled(){rows,err:=db.Query("select id,barber_id,customer_id,sender_role,sender_name,text,sent_at from public.messages order by sent_at");if err==nil{defer rows.Close();var all []models.Message;for rows.Next(){var v models.Message;if rows.Scan(&v.ID,&v.BarberID,&v.CustomerID,&v.SenderRole,&v.SenderName,&v.Text,&v.SentAt)==nil{all=append(all,v)}};return summariseMessages(all,match)}}
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

func summariseMessages(source []models.Message, match func(models.Message) bool) []Conversation {
	type key struct{barberID,customerID int}
	index:=map[key]*Conversation{}
	for _,message:=range source{if !match(message){continue};id:=key{message.BarberID,message.CustomerID};conversation:=index[id];if conversation==nil{conversation=&Conversation{BarberID:message.BarberID,CustomerID:message.CustomerID};index[id]=conversation};conversation.Count++;if !message.SentAt.Before(conversation.LastAt){conversation.LastAt=message.SentAt;conversation.LastText=message.Text}}
	list:=make([]Conversation,0,len(index));for _,v:=range index{list=append(list,*v)};sort.SliceStable(list,func(i,j int)bool{return list[i].LastAt.After(list[j].LastAt)});return list
}
