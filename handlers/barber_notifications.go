package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"barber-booking/models"
	"barber-booking/storage"
)

// bookingAlert is the small JSON shape the barber's phone needs to raise the
// "new booking" alert. It deliberately omits customer contact details: the
// alert only has to say that a request arrived and where to review it.
type bookingAlert struct {
	ID       int     `json:"id"`
	Customer string  `json:"customer"`
	Service  string  `json:"service"`
	Date     string  `json:"date"`
	Time     string  `json:"time"`
	Price    float64 `json:"price"`
}

// messageAlert is the chat equivalent. It carries the sender's name and the
// message text (or attachment label) so the notification can say who wrote and
// what they said before the recipient opens the thread.
type messageAlert struct {
	ID         int    `json:"id"`
	BarberID   int    `json:"barberId"`
	CustomerID int    `json:"customerId"`
	Sender     string `json:"sender"`
	Text       string `json:"text"`
	Image      string `json:"image,omitempty"`
	Audio      string `json:"audio,omitempty"`
	Video      string `json:"video,omitempty"`
	URL        string `json:"url"`
}

// notificationsFeed is shared by both roles. The booking fields stay empty for
// a customer; a barber receives both bookings and messages.
type notificationsFeed struct {
	Latest        int            `json:"latest"`
	Bookings      []bookingAlert `json:"bookings"`
	LatestMessage int            `json:"latestMessage"`
	Messages      []messageAlert `json:"messages"`
	PushPublicKey string         `json:"pushPublicKey,omitempty"`
}

// BarberNotificationsHandler is the poll target behind the live booking and
// message alerts on a barber's dashboard and phone.
//
// The first call for each feed (no cursor) is a baseline: it reports the newest
// id and no alerts, so opening the app does not replay old requests. Later
// calls pass the id they last saw and receive only newer rows.
func BarberNotificationsHandler(w http.ResponseWriter, r *http.Request, barber models.Barber) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	afterBookingText := strings.TrimSpace(r.URL.Query().Get("after"))
	bookingBaseline := afterBookingText == ""
	afterBookingID := 0
	if !bookingBaseline {
		value, err := strconv.Atoi(afterBookingText)
		if err != nil || value < 0 {
			journeyError(w, "That alert cursor is not valid.", http.StatusBadRequest)

			return
		}
		afterBookingID = value
	}

	afterMessageText := strings.TrimSpace(r.URL.Query().Get("afterMessage"))
	messageBaseline := afterMessageText == ""
	afterMessageID := 0
	if !messageBaseline {
		value, err := strconv.Atoi(afterMessageText)
		if err != nil || value < 0 {
			journeyError(w, "That alert cursor is not valid.", http.StatusBadRequest)

			return
		}
		afterMessageID = value
	}

	latest, pending, err := storage.PendingBookingsForBarberAfter(r.Context(), barber.ID, afterBookingID)
	if err != nil {
		serverError(w, "load barber booking alerts", err)

		return
	}

	alerts := make([]bookingAlert, 0, len(pending))
	if !bookingBaseline {
		for _, booking := range pending {
			alerts = append(alerts, bookingAlert{
				ID:       booking.ID,
				Customer: booking.CustomerName,
				Service:  booking.ServiceName,
				Date:     booking.Date,
				Time:     booking.Time,
				Price:    booking.ServicePrice,
			})
		}
	}

	latestMessage, incoming, err := storage.IncomingMessagesForBarberAfter(r.Context(), barber.ID, afterMessageID)
	if err != nil {
		serverError(w, "load barber message alerts", err)

		return
	}

	messages := make([]messageAlert, 0, len(incoming))
	if !messageBaseline {
		for _, message := range incoming {
			messages = append(messages, newMessageAlert(message, "/barber/chats/"+strconv.Itoa(message.CustomerID)))
		}
	}

	writeNotificationsFeed(w, notificationsFeed{
		Latest:        latest,
		Bookings:      alerts,
		LatestMessage: latestMessage,
		Messages:      messages,
	})
}

// CustomerNotificationsHandler is the customer's message feed. Customers do
// not receive booking alerts from their own requests, so only chat messages
// are returned.
func CustomerNotificationsHandler(w http.ResponseWriter, r *http.Request, customer models.Customer) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	afterText := strings.TrimSpace(r.URL.Query().Get("afterMessage"))
	baseline := afterText == ""
	afterID := 0
	if !baseline {
		value, err := strconv.Atoi(afterText)
		if err != nil || value < 0 {
			journeyError(w, "That alert cursor is not valid.", http.StatusBadRequest)

			return
		}
		afterID = value
	}

	latest, incoming, err := storage.IncomingMessagesForCustomerAfter(r.Context(), customer.ID, afterID)
	if err != nil {
		serverError(w, "load customer message alerts", err)

		return
	}

	messages := make([]messageAlert, 0, len(incoming))
	if !baseline {
		for _, message := range incoming {
			messages = append(messages, newMessageAlert(message, "/customer/chats/"+strconv.Itoa(message.BarberID)))
		}
	}

	writeNotificationsFeed(w, notificationsFeed{
		LatestMessage: latest,
		Messages:      messages,
	})
}

func newMessageAlert(message storage.MessageAlert, url string) messageAlert {
	return messageAlert{
		ID:         message.ID,
		BarberID:   message.BarberID,
		CustomerID: message.CustomerID,
		Sender:     message.SenderName,
		Text:       message.Text,
		Image:      message.Image,
		Audio:      message.Audio,
		Video:      message.Video,
		URL:        url,
	}
}

func writeNotificationsFeed(w http.ResponseWriter, feed notificationsFeed) {
	feed.PushPublicKey = PushPublicKey()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(feed)
}
