package handlers

import (
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// maxMessageLength keeps one chat message to a sensible size.
const maxMessageLength = 1000

// barberChatView is used by the barber's individual chat page.
type barberChatView struct {
	chatView
	CustomerImage string
}

// BarberChatsHandler lists the customers who have messaged this barber.
func BarberChatsHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	conversations := storage.ConversationsForBarber(barber.ID)
	rows := make([]chatRow, 0, len(conversations))

	for _, conversation := range conversations {
		row := chatRow{
			BarberID:   conversation.BarberID,
			CustomerID: conversation.CustomerID,
			Name:       "Customer",
			LastText:   conversation.LastText,
			LastAt:     formatWhen(conversation.LastAt),
			Count:      conversation.Count,
		}

		if customer, found := storage.FindCustomerByID(conversation.CustomerID); found {
			row.Name = customer.FullName
			row.Detail = customer.PhoneNumber
		}

		rows = append(rows, row)
	}

	render(w, "barber/chats.html", listView[chatRow]{
		Items: rows,
		CSRF:  s.CSRFToken,
	})
}

// BarberChatHandler shows and appends to one conversation. The customer
// ID comes from the URL and is checked against storage, so a barber
// cannot open a thread with an account that does not exist.
func BarberChatHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	customerSegment string,
) {
	customerID, ok := parseID(customerSegment)
	if !ok {
		http.Error(w, "Conversation not found.", http.StatusNotFound)

		return
	}

	customer, found := storage.FindCustomerByID(customerID)
	if !found {
		http.Error(w, "Conversation not found.", http.StatusNotFound)

		return
	}

	switch r.Method {

	case http.MethodGet:
		renderChat(w, s, barber, customer, "")

	case http.MethodPost:
		if !requireCSRF(w, r, s) {
			return
		}

		text, problem := readMessage(r)
		if problem != "" {
			renderChat(w, s, barber, customer, problem)

			return
		}

		_, err := storage.AddMessage(models.Message{
			BarberID:    barber.ID,
			CustomerID:  customer.ID,
			SenderRole:  models.SenderBarber,
			SenderName:  barber.FullName,
			Text:        text,
		})

		if err != nil {
			serverError(w, "send message", err)

			return
		}

		http.Redirect(
			w,
			r,
			chatPath("/barber/chats/", customer.ID),
			http.StatusSeeOther,
		)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// renderChat draws the barber's side of a thread.
func renderChat(
	w http.ResponseWriter,
	s sessionData,
	barber models.Barber,
	customer models.Customer,
	problem string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	renderStatus(
		w,
		status,
		"barber/chat.html",
		barberChatView{
			chatView: chatView{
				Messages: storage.ListConversation(
					barber.ID,
					customer.ID,
				),
				Title:    customer.FullName,
				Subtitle: customer.PhoneNumber,
				BackLink: "/barber/chats",
				Action:   chatPath("/barber/chats/", customer.ID),
				CSRF:     s.CSRFToken,
				Error:    problem,
			},
			CustomerImage: customer.ProfileImage,
		},
	)
}