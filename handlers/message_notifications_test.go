package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

type messageAlertBody struct {
	LatestMessage int `json:"latestMessage"`
	Messages      []struct {
		ID       int    `json:"id"`
		BarberID int    `json:"barberId"`
		Customer int    `json:"customerId"`
		Sender   string `json:"sender"`
		Text     string `json:"text"`
		URL      string `json:"url"`
	} `json:"messages"`
}

func readMessageAlerts(t *testing.T, response *httptest.ResponseRecorder) messageAlertBody {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var body messageAlertBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode alerts: %v", err)
	}
	return body
}

func addChatMessage(t *testing.T, barber models.Barber, customer models.Customer, senderRole, senderName, text string) models.Message {
	t.Helper()
	message, err := storage.AddMessage(context.Background(), models.Message{
		BarberID:   barber.ID,
		CustomerID: customer.ID,
		SenderRole: senderRole,
		SenderName: senderName,
		Text:       text,
	})
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func TestBarberMessageAlertBaselineThenCustomerMessage(t *testing.T) {
	barber, customer, _ := alertFixture(t)

	// The barber's own message must not alert the barber, but it still moves
	// the baseline so the first poll never replays an old thread.
	own := addChatMessage(t, barber, customer, models.SenderBarber, barber.FullName, "Hello, how can I help?")

	baseline := httptest.NewRecorder()
	BarberNotificationsHandler(baseline, httptest.NewRequest(http.MethodGet, "/barber/notifications", nil), barber)
	body := readMessageAlerts(t, baseline)
	if body.LatestMessage != own.ID {
		t.Fatalf("baseline message latest = %d, want %d", body.LatestMessage, own.ID)
	}
	if len(body.Messages) != 0 {
		t.Fatalf("baseline replayed %d messages", len(body.Messages))
	}

	incoming := addChatMessage(t, barber, customer, models.SenderCustomer, customer.FullName, "I will arrive at 10.")

	response := httptest.NewRecorder()
	BarberNotificationsHandler(response, httptest.NewRequest(http.MethodGet, "/barber/notifications?afterMessage="+strconv.Itoa(own.ID), nil), barber)
	body = readMessageAlerts(t, response)

	if body.LatestMessage != incoming.ID {
		t.Fatalf("latest = %d, want %d", body.LatestMessage, incoming.ID)
	}
	if len(body.Messages) != 1 {
		t.Fatalf("got %d messages, want 1: %+v", len(body.Messages), body.Messages)
	}
	alert := body.Messages[0]
	if alert.Sender != customer.FullName {
		t.Errorf("sender = %q, want %q", alert.Sender, customer.FullName)
	}
	if alert.Text != "I will arrive at 10." {
		t.Errorf("text = %q", alert.Text)
	}
	if alert.Customer != customer.ID {
		t.Errorf("customer id = %d, want %d", alert.Customer, customer.ID)
	}
	if alert.URL != "/barber/chats/"+strconv.Itoa(customer.ID) {
		t.Errorf("url = %q", alert.URL)
	}
}

func TestBarberMessageAlertIgnoresAnotherBarbersThread(t *testing.T) {
	barber, customer, _ := alertFixture(t)
	other, err := storage.CreateBarber(models.Barber{
		Email:    "other_chat_barber@example.com",
		FullName: "Other Chat Barber",
		Status:   models.BarberActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	addChatMessage(t, other, customer, models.SenderCustomer, customer.FullName, "This is for the other barber")

	response := httptest.NewRecorder()
	BarberNotificationsHandler(response, httptest.NewRequest(http.MethodGet, "/barber/notifications?afterMessage=0", nil), barber)
	body := readMessageAlerts(t, response)
	if len(body.Messages) != 0 {
		t.Fatalf("leaked %d messages from another barber's thread", len(body.Messages))
	}
}

func TestCustomerMessageAlertBaselineThenBarberMessage(t *testing.T) {
	barber, customer, _ := alertFixture(t)

	own := addChatMessage(t, barber, customer, models.SenderCustomer, customer.FullName, "Is Saturday free?")

	baseline := httptest.NewRecorder()
	CustomerNotificationsHandler(baseline, httptest.NewRequest(http.MethodGet, "/customer/notifications", nil), customer)
	body := readMessageAlerts(t, baseline)
	if body.LatestMessage != own.ID {
		t.Fatalf("baseline message latest = %d, want %d", body.LatestMessage, own.ID)
	}
	if len(body.Messages) != 0 {
		t.Fatalf("baseline replayed %d messages", len(body.Messages))
	}

	reply := addChatMessage(t, barber, customer, models.SenderBarber, barber.FullName, "Yes, 10:00 works.")

	response := httptest.NewRecorder()
	CustomerNotificationsHandler(response, httptest.NewRequest(http.MethodGet, "/customer/notifications?afterMessage="+strconv.Itoa(own.ID), nil), customer)
	body = readMessageAlerts(t, response)

	if body.LatestMessage != reply.ID {
		t.Fatalf("latest = %d, want %d", body.LatestMessage, reply.ID)
	}
	if len(body.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(body.Messages))
	}
	alert := body.Messages[0]
	if alert.Sender != barber.FullName {
		t.Errorf("sender = %q, want %q", alert.Sender, barber.FullName)
	}
	if alert.Text != "Yes, 10:00 works." {
		t.Errorf("text = %q", alert.Text)
	}
	if alert.BarberID != barber.ID {
		t.Errorf("barber id = %d, want %d", alert.BarberID, barber.ID)
	}
	if alert.URL != "/customer/chats/"+strconv.Itoa(barber.ID) {
		t.Errorf("url = %q", alert.URL)
	}
}

func pushKey(t *testing.T, size int) string {
	t.Helper()
	raw := make([]byte, size)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestPushSubscriptionHandlerStoresOnlyForSignedInUser(t *testing.T) {
	barber, _, _ := alertFixture(t)
	s := sessionData{CSRFToken: "push-token"}
	body := map[string]any{
		"endpoint": "https://fcm.googleapis.com/fcm/send/test-subscription-1",
		"keys": map[string]string{
			"p256dh": pushKey(t, 65),
			"auth":   pushKey(t, 16),
		},
	}
	encoded, _ := json.Marshal(body)

	request := httptest.NewRequest(http.MethodPost, "/barber/notifications/subscribe", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "push-token")
	response := httptest.NewRecorder()
	BarberPushSubscriptionHandler(response, request, barber, s)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}

	subscriptions, err := storage.PushSubscriptionsFor(context.Background(), "barber", barber.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subscriptions) != 1 || subscriptions[0].Endpoint != body["endpoint"] {
		t.Fatalf("subscriptions = %+v", subscriptions)
	}

	deleteBody, _ := json.Marshal(map[string]string{"endpoint": body["endpoint"].(string)})
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/barber/notifications/subscribe", bytes.NewReader(deleteBody))
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRequest.Header.Set("X-CSRF-Token", "push-token")
	deleteResponse := httptest.NewRecorder()
	BarberPushSubscriptionHandler(deleteResponse, deleteRequest, barber, s)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}

	subscriptions, err = storage.PushSubscriptionsFor(context.Background(), "barber", barber.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subscriptions) != 0 {
		t.Fatalf("subscription was not removed: %+v", subscriptions)
	}
}

func TestPushSubscriptionHandlerRejectsMissingCSRFAndBadEndpoint(t *testing.T) {
	barber, _, _ := alertFixture(t)
	s := sessionData{CSRFToken: "push-token"}

	request := httptest.NewRequest(http.MethodPost, "/barber/notifications/subscribe", bytes.NewReader([]byte(`{}`)))
	response := httptest.NewRecorder()
	BarberPushSubscriptionHandler(response, request, barber, s)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, want 403", response.Code)
	}

	encoded, _ := json.Marshal(map[string]any{
		"endpoint": "http://not-a-real-push-service.example/subscription",
		"keys": map[string]string{
			"p256dh": pushKey(t, 65),
			"auth":   pushKey(t, 16),
		},
	})
	request = httptest.NewRequest(http.MethodPost, "/barber/notifications/subscribe", bytes.NewReader(encoded))
	request.Header.Set("X-CSRF-Token", "push-token")
	response = httptest.NewRecorder()
	BarberPushSubscriptionHandler(response, request, barber, s)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("bad endpoint status = %d, want 400", response.Code)
	}
}

func TestMessagePushCarriesSenderAndMessage(t *testing.T) {
	notification := messagePush("Ada Customer", "I am on my way", "", "", "", "/barber/chats/7", 42)
	if notification.Title != "New message from Ada Customer" {
		t.Errorf("title = %q", notification.Title)
	}
	if notification.Body != "Ada Customer: I am on my way" {
		t.Errorf("body = %q", notification.Body)
	}
	if notification.URL != "/barber/chats/7" {
		t.Errorf("url = %q", notification.URL)
	}
	if notification.Tag != "chat-message-42" {
		t.Errorf("tag = %q", notification.Tag)
	}

	voice := messagePush("Alerts Barber", "", "", "voice.m4a", "", "/customer/chats/3", 43)
	if voice.Body != "Alerts Barber: Sent a voice note" {
		t.Errorf("voice body = %q", voice.Body)
	}
}

func TestBookingPushCarriesCustomerAndService(t *testing.T) {
	notification := bookingPush(9, "Ada Customer", "Low Fade", "2026-11-20", "10:00")
	if notification.Title != "New booking from Ada Customer" {
		t.Errorf("title = %q", notification.Title)
	}
	if notification.Body != "Ada Customer booked Low Fade · 2026-11-20 at 10:00" {
		t.Errorf("body = %q", notification.Body)
	}
	if notification.URL != "/barber/bookings" {
		t.Errorf("url = %q", notification.URL)
	}
}
