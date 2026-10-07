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

func TestIdentityForms(t *testing.T) {
	valid := url.Values{"fullname": {"  Ọlá O'Neil  "}, "email": {"  USER@EXAMPLE.COM  "}, "phone": {"+234 (801) 234-5678"}, "shopname": {"Shop"}, "shopaddress": {"12 Example Street"}, "state": {"Lagos"}, "local_area": {"Ikeja"}}
	for _, test := range []struct {
		name, field, value string
		barberOnly         bool
	}{
		{"valid international name", "fullname", "Ọlá O'Neil", false},
		{"long name", "fullname", strings.Repeat("名", 121), false},
		{"control in name", "fullname", "Buyer\nName", false},
		{"invalid UTF8", "fullname", "Buyer\xff", false},
		{"invalid email", "email", "buyer@", false},
		{"display name email", "email", "Buyer <buyer@example.com>", false},
		{"long email", "email", strings.Repeat("a", 65) + "@example.com", false},
		{"short phone", "phone", "123", false},
		{"long phone", "phone", strings.Repeat("1", 16), false},
		{"phone text", "phone", "08012abc678", false},
		{"embedded plus", "phone", "080+1234567", false},
		{"missing state", "state", "  ", true},
		{"missing area", "local_area", "", true},
		{"long state", "state", strings.Repeat("a", 101), true},
		{"long area", "local_area", strings.Repeat("a", 101), true},
		{"long shop", "shopname", strings.Repeat("a", 161), true},
		{"long address", "shopaddress", strings.Repeat("a", 301), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := url.Values{}
			for key, value := range valid {
				values[key] = append([]string(nil), value...)
			}
			values.Set(test.field, test.value)
			read := func(barber bool) string {
				r := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				if barber {
					_, problem := readBarberForm(r)
					return problem
				}
				_, problem := readCustomerForm(r)
				return problem
			}
			wantProblem := test.name != "valid international name"
			if problem := read(true); (problem != "") != wantProblem {
				t.Fatalf("barber problem = %q", problem)
			}
			if !test.barberOnly {
				if problem := read(false); (problem != "") != wantProblem {
					t.Fatalf("customer problem = %q", problem)
				}
			}
		})
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(valid.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	form, problem := readBarberForm(request)
	if problem != "" || form.Email != "user@example.com" || form.FullName != "Ọlá O'Neil" {
		t.Fatalf("normalization: %+v, %q", form, problem)
	}
}

func TestInvalidIdentityMutationsPreserveAccounts(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	barber, err := storage.CreateBarber(models.Barber{FullName: "Original Barber", Email: "original@example.com", PhoneNumber: "08012345678", ShopName: "Original shop", ShopAddress: "Original address", State: "Lagos", LocalArea: "Ikeja", Status: models.BarberActive})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{FullName: "Original Customer", Email: "customer@example.com", PhoneNumber: "08012345678"})
	if err != nil {
		t.Fatal(err)
	}
	s := sessionData{CSRFToken: "test-csrf"}
	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
		values  url.Values
	}{
		{"barber signup", BarberRegisterHandler, url.Values{"fullname": {"New Barber"}, "email": {"new@example.com"}, "phone": {"08012345678"}, "shopname": {"Shop"}, "shopaddress": {"Address"}, "local_area": {"Ikeja"}}},
		{"customer signup", CustomerRegisterHandler, url.Values{"fullname": {"New Customer"}, "email": {"invalid-email"}, "phone": {"08012345678"}}},
		{"barber profile", func(w http.ResponseWriter, r *http.Request) { EditProfileHandler(w, r, barber, s) }, url.Values{"fullname": {"Changed Barber"}, "email": {barber.Email}, "phone": {barber.PhoneNumber}, "shopname": {"Shop"}, "shopaddress": {"Address"}, "state": {"Lagos"}}},
		{"admin create", func(w http.ResponseWriter, r *http.Request) { NewBarberHandler(w, r, s) }, url.Values{"fullname": {"Admin Barber"}, "email": {"admin-new@example.com"}, "phone": {"08012345678"}, "shopname": {"Shop"}, "shopaddress": {"Address"}, "local_area": {"Ikeja"}}},
		{"admin update", func(w http.ResponseWriter, r *http.Request) { UpdateBarberHandler(w, r, s, strconv.Itoa(barber.ID)) }, url.Values{"fullname": {"Changed Barber"}, "email": {barber.Email}, "phone": {barber.PhoneNumber}, "shopname": {"Shop"}, "shopaddress": {"Address"}, "local_area": {"Ikeja"}}},
		{"customer profile", func(w http.ResponseWriter, r *http.Request) { EditCustomerProfileHandler(w, r, customer, s) }, url.Values{"fullname": {"Changed Customer"}, "email": {customer.Email}, "phone": {"123"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.values.Set("csrf_token", s.CSRFToken)
			test.values.Set("password", "valid-password")
			test.values.Set("confirm_password", "valid-password")
			response := httptest.NewRecorder()
			test.handler(response, authFormRequest("/", test.values))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid mutation status = %d: %s", response.Code, response.Body.String())
			}
			storedBarbers := storage.ListBarbers()
			storedCustomers := storage.ListCustomers()
			if len(storedBarbers) != 1 || storedBarbers[0] != barber || len(storedCustomers) != 1 || storedCustomers[0] != customer {
				t.Fatal("invalid mutation changed stored accounts")
			}
		})
	}
}
