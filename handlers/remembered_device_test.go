package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/security"
	"barber-booking/storage"

	"github.com/go-webauthn/webauthn/webauthn"
)

func createLoginCustomer(t *testing.T, email, password string) {
	t.Helper()
	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateCustomerContext(context.Background(), models.Customer{
		FullName:     "Ada Buyer",
		PhoneNumber:  "08012345678",
		Email:        email,
		PasswordHash: hash,
	}); err != nil {
		t.Fatal(err)
	}
}

func createLoginBarber(t *testing.T, email, password string) {
	t.Helper()
	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateBarberContext(context.Background(), models.Barber{
		FullName:     "Bob Barber",
		PhoneNumber:  "08012345679",
		Email:        email,
		PasswordHash: hash,
		ShopName:     "Bob's Chair",
		ShopAddress:  "12 High Street",
		State:        "Lagos",
		LocalArea:    "Ikeja",
		Status:       models.BarberActive,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRememberedCustomerLoginAsksOnlyForPassword(t *testing.T) {
	server := newServer(t)
	createLoginCustomer(t, "ada@example.com", "correct horse battery")
	browser := newBrowser(t)

	post(t, browser, server.URL+"/customer/login", url.Values{
		"email":    {"ada@example.com"},
		"password": {"correct horse battery"},
	})

	page := get(t, browser, server.URL+"/customer/login")
	mustContain(t, "remembered login", page, "Signing in as")
	mustContain(t, "remembered login", page, "ada@example.com")
	mustContain(t, "remembered login", page, "Use another account")
	mustContain(t, "remembered login", page, `name="email" value="ada@example.com"`)
	if strings.Contains(page, `id="email"`) {
		t.Fatal("remembered login still asks for the email address")
	}

	response, err := browser.PostForm(server.URL+"/customer/login", url.Values{
		"email":      {"ada@example.com"},
		"password":   {"wrong password"},
		"remembered": {"1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", response.StatusCode)
	}
	rejected := string(body)
	mustContain(t, "rejected remembered login", rejected, "Invalid email or password.")
	mustContain(t, "rejected remembered login", rejected, "Signing in as")
	if strings.Contains(rejected, `id="email"`) {
		t.Fatal("rejected remembered login fell back to the email field")
	}

	switched := get(t, browser, server.URL+"/customer/login?switch=1")
	if strings.Contains(switched, "Signing in as") {
		t.Fatal("switch did not forget the remembered account")
	}
	mustContain(t, "switched login", switched, `id="email"`)
}

func TestRememberedBarberLoginAsksOnlyForPassword(t *testing.T) {
	server := newServer(t)
	createLoginBarber(t, "bob@example.com", "clippers and all")
	browser := newBrowser(t)

	post(t, browser, server.URL+"/barber/login", url.Values{
		"email":    {"bob@example.com"},
		"password": {"clippers and all"},
	})

	page := get(t, browser, server.URL+"/barber/login")
	mustContain(t, "remembered barber login", page, "Signing in as")
	mustContain(t, "remembered barber login", page, "bob@example.com")
	if strings.Contains(page, `id="email"`) {
		t.Fatal("remembered barber login still asks for the email address")
	}

	switched := get(t, browser, server.URL+"/barber/login?switch=1")
	if strings.Contains(switched, "Signing in as") {
		t.Fatal("barber switch did not forget the remembered account")
	}
	mustContain(t, "switched barber login", switched, `id="email"`)
}

func TestPasskeyLoginNeedsRememberedDeviceAndRegisteredCredential(t *testing.T) {
	server := newServer(t)
	createLoginCustomer(t, "ada@example.com", "correct horse battery")
	browser := newBrowser(t)

	response, err := browser.Post(server.URL+"/customer/passkey/login/begin", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("passkey without remembered account = %d", response.StatusCode)
	}

	post(t, browser, server.URL+"/customer/login", url.Values{
		"email":    {"ada@example.com"},
		"password": {"correct horse battery"},
	})

	response, err = browser.Post(server.URL+"/customer/passkey/login/begin", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("passkey without a credential = %d", response.StatusCode)
	}
}

func TestPasskeyEnrolmentRequiresCSRFHeader(t *testing.T) {
	server := newServer(t)
	createLoginCustomer(t, "ada@example.com", "correct horse battery")
	browser := newBrowser(t)

	post(t, browser, server.URL+"/customer/login", url.Values{
		"email":    {"ada@example.com"},
		"password": {"correct horse battery"},
	})
	token := csrfToken(t, "customer profile edit", get(t, browser, server.URL+"/customer/profile/edit"))

	response, err := browser.Post(server.URL+"/customer/passkey/register/begin", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("passkey begin without CSRF = %d", response.StatusCode)
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+"/customer/passkey/register/begin", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", token)
	response, err = browser.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"publicKey"`) {
		t.Fatalf("passkey begin = %d %s", response.StatusCode, body)
	}
	if _, found := findCookie(response.Cookies(), "customer_passkey_register"); !found {
		t.Fatal("passkey begin did not set a challenge cookie")
	}
}

func TestPasskeyLoginOffersRegisteredCredentials(t *testing.T) {
	server := newServer(t)
	createLoginCustomer(t, "ada@example.com", "correct horse battery")
	browser := newBrowser(t)

	post(t, browser, server.URL+"/customer/login", url.Values{
		"email":    {"ada@example.com"},
		"password": {"correct horse battery"},
	})

	customer, err := storage.FindCustomerByEmailResult(context.Background(), "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	credential, err := json.Marshal(webauthn.Credential{
		ID:        []byte("test-credential-id"),
		PublicKey: []byte{0x01, 0x02, 0x03},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.CreatePasskeyContext(context.Background(), models.Passkey{
		CredentialID: []byte("test-credential-id"),
		Role:         "customer",
		UserID:       customer.ID,
		Email:        customer.Email,
		DisplayName:  customer.FullName,
		Credential:   credential,
	}); err != nil {
		t.Fatal(err)
	}

	page := get(t, browser, server.URL+"/customer/login")
	mustContain(t, "remembered login with passkey", page, "data-passkey-login")

	response, err := browser.Post(server.URL+"/customer/passkey/login/begin", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString([]byte("test-credential-id"))
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), encoded) {
		t.Fatalf("passkey begin = %d %s", response.StatusCode, body)
	}
}

func findCookie(cookies []*http.Cookie, name string) (*http.Cookie, bool) {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie, true
		}
	}
	return nil, false
}
