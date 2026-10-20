package handlers

import (
	"errors"
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

	cursor, ok := readPageCursor(w, r)
	if !ok {
		return
	}
	page, err := storage.ConversationPageForBarber(r.Context(), barber.ID, cursor)
	if err != nil {
		serverError(w, "load barber conversations", err)
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
		rows = append(rows, chatRow{BarberID: conversation.BarberID, CustomerID: conversation.CustomerID, Name: conversation.Name, Detail: conversation.Detail, LastText: conversation.LastText, LastAt: formatWhen(conversation.LastAt), Count: conversation.Count, CustomerImage: conversation.Image})
	}

	render(w, "barber/chats.html", listView[chatRow]{
		Items:      rows,
		Pagination: pagination,
		CSRF:       s.CSRFToken,
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
		journeyError(w, "Conversation not found.", http.StatusNotFound)

		return
	}

	customer, err := storage.FindCustomerByIDResult(r.Context(), customerID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load conversation customer", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Conversation not found.", http.StatusNotFound)

		return
	}

	switch r.Method {

	case http.MethodGet:
		renderChat(w, r, s, barber, customer, "")

	case http.MethodPost:
		extendUploadDeadline(w)
		limitChatRequestBody(w, r)
		if !requireCSRF(w, r, s) {
			return
		}
		defer cleanupMultipart(r)

		text, image, audio, video, problem := readChatMessage(r)
		if problem != "" {
			renderChat(w, r, s, barber, customer, problem)

			return
		}

		_, err := storage.AddMessage(r.Context(), models.Message{
			BarberID:   barber.ID,
			CustomerID: customer.ID,
			SenderRole: models.SenderBarber,
			SenderName: barber.FullName,
			Text:       text,
			Image:      image,
			Audio:      audio,
			Video:      video,
		})

		if err != nil {
			removeChatMedia(image)
			removeChatMedia(audio)
			removeChatMedia(video)
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
	r *http.Request,
	s sessionData,
	barber models.Barber,
	customer models.Customer,
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
		"barber/chat.html",
		barberChatView{
			chatView: chatView{
				Messages:   messages,
				Pagination: pagination,
				Title:      customer.FullName,
				Subtitle:   customer.PhoneNumber,
				BackLink:   "/barber/chats",
				Action:     chatPath("/barber/chats/", customer.ID),
				CSRF:       s.CSRFToken,
				Error:      problem,
			},
			CustomerImage: customer.ProfileImage,
		},
	)
}
