package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// CustomerMessageActionHandler edits or deletes one message the customer
// sent in a conversation with a barber.
func CustomerMessageActionHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	barberSegment, messageSegment, action string,
) {
	barber, ok := bookableBarber(w, r, barberSegment)
	if !ok {
		return
	}

	handleMessageAction(
		w,
		r,
		s,
		models.SenderCustomer,
		barber.ID,
		customer.ID,
		messageSegment,
		action,
		chatPath("/customer/chats/", barber.ID),
		func(problem string) {
			renderCustomerChat(w, r, s, customer, barber, problem)
		},
	)
}

// BarberMessageActionHandler edits or deletes one message the barber sent
// in a conversation with a customer.
func BarberMessageActionHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	customerSegment, messageSegment, action string,
) {
	customerID, ok := parseID(customerSegment)
	if !ok {
		journeyError(w, "Conversation not found.", http.StatusNotFound)

		return
	}

	customer, err := storage.FindCustomerByIDResult(r.Context(), customerID)
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Conversation not found.", http.StatusNotFound)

		return
	}
	if err != nil {
		serverError(w, "load conversation customer", err)

		return
	}

	handleMessageAction(
		w,
		r,
		s,
		models.SenderBarber,
		barber.ID,
		customer.ID,
		messageSegment,
		action,
		chatPath("/barber/chats/", customer.ID),
		func(problem string) {
			renderChat(w, r, s, barber, customer, problem)
		},
	)
}

// handleMessageAction applies an edit or delete to a single message. The
// message must belong to the signed-in account and have been sent by that
// same side of the conversation.
func handleMessageAction(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	senderRole string,
	barberID, customerID int,
	messageSegment, action, threadPath string,
	renderProblem func(string),
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	messageID, ok := parseID(messageSegment)
	if !ok {
		journeyError(w, "Message not found.", http.StatusNotFound)

		return
	}

	message, err := storage.FindMessage(r.Context(), messageID)
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Message not found.", http.StatusNotFound)

		return
	}
	if err != nil {
		serverError(w, "load chat message", err)

		return
	}

	if message.BarberID != barberID ||
		message.CustomerID != customerID ||
		message.SenderRole != senderRole {

		journeyError(w, "Message not found.", http.StatusNotFound)

		return
	}

	switch action {

	case "edit":
		text := trimmedField(r.PostFormValue("text"))
		if len(text) > maxMessageLength {
			text = text[:maxMessageLength]
		}

		if text == "" && !message.HasImage() && !message.HasAudio() && !message.HasVideo() {
			renderProblem("Type a message before saving.")

			return
		}

		err := storage.UpdateMessageText(
			r.Context(),
			message.ID,
			barberID,
			customerID,
			senderRole,
			text,
		)
		if errors.Is(err, storage.ErrNotFound) {
			journeyError(w, "Message not found.", http.StatusNotFound)

			return
		}
		if err != nil {
			serverError(w, "edit chat message", err)

			return
		}

	case "delete":
		removed, err := storage.DeleteMessage(
			r.Context(),
			message.ID,
			barberID,
			customerID,
			senderRole,
		)
		if errors.Is(err, storage.ErrNotFound) {
			journeyError(w, "Message not found.", http.StatusNotFound)

			return
		}
		if err != nil {
			serverError(w, "delete chat message", err)

			return
		}

		removeChatMedia(removed.Image)
		removeChatMedia(removed.Audio)
		removeChatMedia(removed.Video)

	default:
		journeyError(w, "Page not found.", http.StatusNotFound)

		return
	}

	http.Redirect(w, r, threadPath, http.StatusSeeOther)
}
