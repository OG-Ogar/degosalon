package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestPublicCustomerProfileExposesOnlyNameAndPhoto(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	customer, err := storage.CreateCustomer(models.Customer{
		FullName:     "Visible Community Name",
		PhoneNumber:  "08012345678",
		Email:        "private@example.test",
		ProfileImage: "/uploads/profiles/visible.png",
	})
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	PublicCustomerProfileHandler(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/people/%d", customer.ID), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}

	body := response.Body.String()
	for _, want := range []string{"Visible Community Name", "/uploads/profiles/visible.png"} {
		mustContain(t, "public customer profile", body, want)
	}
	for _, secret := range []string{"private@example.test", "08012345678"} {
		if strings.Contains(body, secret) {
			t.Fatalf("public customer profile exposed %q", secret)
		}
	}
}

func TestPublicCustomerProfileHidesMissingAccount(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	response := httptest.NewRecorder()
	PublicCustomerProfileHandler(response, httptest.NewRequest(http.MethodGet, "/people/9999", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status %d", response.Code)
	}
}
