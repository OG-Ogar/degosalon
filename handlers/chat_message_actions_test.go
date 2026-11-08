package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

func messageActionFixtures(t *testing.T) (models.Barber, models.Customer, models.Message) {
	t.Helper()

	storage.Reset()
	t.Cleanup(storage.Reset)

	barber, err := storage.CreateBarber(models.Barber{Email: "actions@example.com", FullName: "Actions Barber"})
	if err != nil {
		t.Fatalf("create barber: %v", err)
	}
	customer, err := storage.CreateCustomer(models.Customer{Email: "actions-customer@example.com", FullName: "Actions Customer"})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}

	message, err := storage.AddMessage(context.Background(), models.Message{
		BarberID:   barber.ID,
		CustomerID: customer.ID,
		SenderRole: models.SenderCustomer,
		SenderName: customer.FullName,
		Text:       "see you at five",
	})
	if err != nil {
		t.Fatalf("add message: %v", err)
	}

	return barber, customer, message
}

func messageActionRequest(t *testing.T, path, text string) *http.Request {
	t.Helper()

	form := url.Values{"csrf_token": {"csrf-test"}}
	if text != "" {
		form.Set("text", text)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

func TestCustomerCanEditOwnMessage(t *testing.T) {
	barber, customer, message := messageActionFixtures(t)
	s := sessionData{CSRFToken: "csrf-test"}
	path := "/customer/chats/" + strconv.Itoa(barber.ID) + "/messages/" + strconv.Itoa(message.ID) + "/edit"
	response := httptest.NewRecorder()

	CustomerMessageActionHandler(response, messageActionRequest(t, path, "see you at six"), customer, s, strconv.Itoa(barber.ID), strconv.Itoa(message.ID), "edit")

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusSeeOther, response.Body.String())
	}

	updated, err := storage.FindMessage(context.Background(), message.ID)
	if err != nil {
		t.Fatalf("find message: %v", err)
	}
	if updated.Text != "see you at six" {
		t.Fatalf("text = %q, want %q", updated.Text, "see you at six")
	}
}

func TestCustomerCannotEditBarberMessage(t *testing.T) {
	barber, customer, _ := messageActionFixtures(t)

	barberMessage, err := storage.AddMessage(context.Background(), models.Message{
		BarberID:   barber.ID,
		CustomerID: customer.ID,
		SenderRole: models.SenderBarber,
		SenderName: barber.FullName,
		Text:       "barber reply",
	})
	if err != nil {
		t.Fatalf("add barber message: %v", err)
	}

	s := sessionData{CSRFToken: "csrf-test"}
	path := "/customer/chats/" + strconv.Itoa(barber.ID) + "/messages/" + strconv.Itoa(barberMessage.ID) + "/edit"
	response := httptest.NewRecorder()

	CustomerMessageActionHandler(response, messageActionRequest(t, path, "hijacked"), customer, s, strconv.Itoa(barber.ID), strconv.Itoa(barberMessage.ID), "edit")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}

	unchanged, err := storage.FindMessage(context.Background(), barberMessage.ID)
	if err != nil {
		t.Fatalf("find message: %v", err)
	}
	if unchanged.Text != "barber reply" {
		t.Fatalf("text = %q, want it unchanged", unchanged.Text)
	}
}

func TestCustomerCanDeleteOwnMessage(t *testing.T) {
	barber, customer, message := messageActionFixtures(t)
	s := sessionData{CSRFToken: "csrf-test"}
	path := "/customer/chats/" + strconv.Itoa(barber.ID) + "/messages/" + strconv.Itoa(message.ID) + "/delete"
	response := httptest.NewRecorder()

	CustomerMessageActionHandler(response, messageActionRequest(t, path, ""), customer, s, strconv.Itoa(barber.ID), strconv.Itoa(message.ID), "delete")

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusSeeOther, response.Body.String())
	}

	if _, err := storage.FindMessage(context.Background(), message.ID); err == nil {
		t.Fatal("message still exists after delete")
	}
}

func TestEditingToEmptyTextIsRejected(t *testing.T) {
	barber, customer, message := messageActionFixtures(t)
	s := sessionData{CSRFToken: "csrf-test"}
	path := "/customer/chats/" + strconv.Itoa(barber.ID) + "/messages/" + strconv.Itoa(message.ID) + "/edit"
	response := httptest.NewRecorder()

	CustomerMessageActionHandler(response, messageActionRequest(t, path, ""), customer, s, strconv.Itoa(barber.ID), strconv.Itoa(message.ID), "edit")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	unchanged, err := storage.FindMessage(context.Background(), message.ID)
	if err != nil {
		t.Fatalf("find message: %v", err)
	}
	if unchanged.Text != "see you at five" {
		t.Fatalf("text = %q, want it unchanged", unchanged.Text)
	}
}

func TestChatThreadRendersMessageActionHooks(t *testing.T) {
	if err := LoadTemplates("../templates"); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	render(response, "customer/chat.html", chatView{
		Title:  "Barber",
		Action: "/customer/chats/1",
		CSRF:   "csrf-test",
		Messages: []models.Message{
			{ID: 7, SenderRole: models.SenderCustomer, SenderName: "Client", Text: "hello", SentAt: time.Now()},
		},
	})

	body := response.Body.String()
	for _, want := range []string{
		`data-message-own`,
		`data-message-edit-url="/customer/chats/1/messages/7/edit"`,
		`data-message-delete-url="/customer/chats/1/messages/7/delete"`,
		`data-message-menu-trigger`,
		`data-message-dialog`,
		`data-message-edit-form`,
		`data-message-delete-form`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("chat page missing %q", want)
		}
	}
}

func TestMessageActionRejectsBadCSRF(t *testing.T) {
	barber, customer, message := messageActionFixtures(t)
	s := sessionData{CSRFToken: "csrf-test"}
	path := "/customer/chats/" + strconv.Itoa(barber.ID) + "/messages/" + strconv.Itoa(message.ID) + "/delete"

	form := url.Values{"csrf_token": {"wrong"}, "text": {"x"}}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	CustomerMessageActionHandler(response, req, customer, s, strconv.Itoa(barber.ID), strconv.Itoa(message.ID), "delete")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}

	if _, err := storage.FindMessage(context.Background(), message.ID); err != nil {
		t.Fatalf("message was deleted despite bad CSRF: %v", err)
	}
}
