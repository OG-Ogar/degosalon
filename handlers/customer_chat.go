package handlers

import (
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// CustomerChatsHandler lists the barbers this customer has messaged.
func CustomerChatsHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	conversations := storage.ConversationsForCustomer(customer.ID)
	rows := make([]chatRow, 0, len(conversations))

	for _, conversation := range conversations {
		row := chatRow{
			BarberID:   conversation.BarberID,
			CustomerID: conversation.CustomerID,
			Name:       "Barber",
			LastText:   conversation.LastText,
			LastAt:     formatWhen(conversation.LastAt),
			Count:      conversation.Count,
		}

		if barber, found := storage.FindBarberByID(conversation.BarberID); found {
			row.Name = barber.FullName
			row.Detail = barber.ShopName
		}

		rows = append(rows, row)
	}

	render(w, "customer/chats.html", listView[chatRow]{
		Items: rows,
		CSRF:  s.CSRFToken,
	})
}

// CustomerChatHandler shows and appends to one conversation with a
// barber. A customer may start a chat with any active barber, which is
// how the "message this barber" button on the shop page works.
func CustomerChatHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	barberSegment string,
) {
	barber, ok := bookableBarber(w, barberSegment)
	if !ok {
		return
	}

	switch r.Method {

	case http.MethodGet:
		renderCustomerChat(w, s, customer, barber, "")

	case http.MethodPost:
		if !requireCSRF(w, r, s) {
			return
		}

		text, problem := readMessage(r)
		if problem != "" {
			renderCustomerChat(w, s, customer, barber, problem)

			return
		}

		_, err := storage.AddMessage(models.Message{
			BarberID:   barber.ID,
			CustomerID: customer.ID,
			SenderRole: models.SenderCustomer,
			SenderName: customer.FullName,
			Text:       text,
		})

		if err != nil {
			serverError(w, "send message", err)

			return
		}

		http.Redirect(w, r, chatPath("/customer/chats/", barber.ID), http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// renderCustomerChat draws the customer's side of a thread.
func renderCustomerChat(
	w http.ResponseWriter,
	s sessionData,
	customer models.Customer,
	barber models.Barber,
	problem string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	renderStatus(
		w,
		status,
		"customer/chat.html",
		chatView{
			Messages: storage.ListConversation(
				barber.ID,
				customer.ID,
			),
			Title:       barber.FullName,
			Subtitle:    barber.ShopName,
			BarberImage: barber.PersonalImage,
			BackLink:    "/customer/chats",
			Action: chatPath(
				"/customer/chats/",
				barber.ID,
			),
			CSRF:  s.CSRFToken,
			Error: problem,
		},
	)
}
