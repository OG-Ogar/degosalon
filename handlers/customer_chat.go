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

	cursor, ok := readPageCursor(w, r)
	if !ok {
		return
	}
	page, err := storage.ConversationPageForCustomer(r.Context(), customer.ID, cursor)
	if err != nil {
		serverError(w, "load customer conversations", err)
		return
	}
	pagination := paginationLinks(r, cursor, 0, 0, false, false, false)
	if len(page.Items) > 0 {
		pagination = paginationLinks(r, cursor, page.Items[0].LastID, page.Items[len(page.Items)-1].LastID, false, page.HasPrevious, page.HasNext)
	} else if cursor.After > 0 || cursor.Before > 0 {
		http.Redirect(w, r, pagination.FirstLink, http.StatusSeeOther)
		return
	}
	rows := make([]chatRow, 0, len(page.Items))
	for _, conversation := range page.Items {
		rows = append(rows, chatRow{BarberID: conversation.BarberID, CustomerID: conversation.CustomerID, Name: conversation.Name, Detail: conversation.Detail, LastText: conversation.LastText, LastAt: formatWhen(conversation.LastAt), Count: conversation.Count, BarberImage: conversation.Image})
	}

	render(w, "customer/chats.html", listView[chatRow]{
		Items:      rows,
		Pagination: pagination,
		CSRF:       s.CSRFToken,
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

	messages, pagination, ok := renderMessagePage(w, r, barber.ID, customer.ID)
	if !ok {
		return
	}

	renderStatus(
		w,
		status,
		"customer/chat.html",
		chatView{
			Messages:    messages,
			Pagination:  pagination,
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
