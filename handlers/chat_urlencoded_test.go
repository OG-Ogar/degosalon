package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

// A plain application/x-www-form-urlencoded chat post carries no file at all.
// The optional photo/voice fields must be read as absent, not rejected: an
// ordinary form submit (as Playwright's request.post(form: …) sends) used to
// fail with 400 because r.FormFile reports http.ErrNotMultipart for a
// non-multipart body.
func TestReadChatMessageAcceptsUrlencodedTextOnly(t *testing.T) {
	body := url.Values{"text": {"hello from a plain form"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/customer/chats/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	text, image, audio, problem := readChatMessage(req)

	if problem != "" {
		t.Fatalf("urlencoded text-only post rejected: %q", problem)
	}
	if text != "hello from a plain form" {
		t.Fatalf("text = %q, want %q", text, "hello from a plain form")
	}
	if image != "" || audio != "" {
		t.Fatalf("attachments = %q/%q, want empty", image, audio)
	}
}

// An empty urlencoded post is still rejected, so accepting non-multipart
// bodies does not let blank messages through.
func TestReadChatMessageRejectsEmptyUrlencoded(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/customer/chats/1", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if _, _, _, problem := readChatMessage(req); problem == "" {
		t.Fatal("empty post accepted, want a validation problem")
	}
}

// The full handler must answer a urlencoded send with the usual 303 redirect
// rather than the 400 that the missing-attachment path used to produce.
func TestCustomerChatAcceptUrlencodedTextOnlyPost(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	barber, err := storage.CreateBarber(models.Barber{Email: "chat@example.com", FullName: "Chat Barber"})
	if err != nil {
		t.Fatalf("create barber: %v", err)
	}

	customer, err := storage.CreateCustomer(models.Customer{Email: "chat-customer@example.com", FullName: "Chat Customer"})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}

	s := sessionData{CSRFToken: "csrf-test"}

	form := url.Values{"csrf_token": {"csrf-test"}, "text": {"hello from a plain form"}}
	req := httptest.NewRequest(
		http.MethodPost,
		"/customer/chats/"+strconv.Itoa(barber.ID),
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	CustomerChatHandler(response, req, customer, s, strconv.Itoa(barber.ID))

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusSeeOther, response.Body.String())
	}
	if got := response.Header().Get("Location"); got != "/customer/chats/"+strconv.Itoa(barber.ID) {
		t.Fatalf("Location = %q", got)
	}
}
