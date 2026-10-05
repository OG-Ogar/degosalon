package handlers

import (
	"errors"
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

	conversations, err := storage.ConversationsForCustomer(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load customer conversations", err)
		return
	}

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

		barber, err := storage.FindBarberByIDContext(r.Context(), conversation.BarberID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "load conversation participant", err)
			return
		}
		if err == nil {
			row.Name = barber.FullName
			row.Detail = barber.ShopName
			row.BarberImage = barber.PersonalImage
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
	barber, ok := bookableBarber(w, r, barberSegment)
	if !ok {
		return
	}

	switch r.Method {

	case http.MethodGet:
		renderCustomerChat(w, r, s, customer, barber, "")

	case http.MethodPost:
		if !requireCSRF(w, r, s) {
			return
		}

		text, problem := readMessage(r)
		if problem != "" {
			renderCustomerChat(w, r, s, customer, barber, problem)

			return
		}

		_, err := storage.AddMessage(r.Context(), models.Message{
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

		http.Redirect(
			w,
			r,
			chatPath("/customer/chats/", barber.ID),
			http.StatusSeeOther,
		)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// renderCustomerChat draws the customer's side of a thread.
func renderCustomerChat(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	customer models.Customer,
	barber models.Barber,
	problem string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	messages, err := storage.ListConversation(r.Context(), barber.ID, customer.ID)
	if err != nil {
		serverError(w, "load conversation messages", err)
		return
	}
	renderStatus(
		w,
		status,
		"customer/chat.html",
		chatView{
			Messages:    messages,
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
