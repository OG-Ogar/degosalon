package handlers

import (
	"bytes"
	"html"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
)

func TestCustomerReturnDestinations(t *testing.T) {
	valid := "/customer/barbers/12?state=Lagos&area=Ikeja&hairstyle=3#booking"
	parsed, _ := url.Parse(valid)
	if got := safeCustomerNext(valid); got != parsed.String() && got != "/customer/barbers/12?area=Ikeja&hairstyle=3&state=Lagos#booking" {
		t.Fatalf("valid destination dropped: %s", got)
	}
	for _, value := range []string{"https://example.com/customer/barbers", "//example.com/customer/barbers", "/customer/../admin", "/customer/%2e%2e/admin", "/customer/barbers%2f12", "/customer/login", "/customer/register", "/customer/logout", "/customer/products/pay", "/customer/barbers/12/reviews", "/customer/\\evil"} {
		if got := safeCustomerNext(value); got != "" {
			t.Fatalf("unsafe destination %q accepted as %q", value, got)
		}
	}
}

func TestDiscoveryContextSurvivesAuthentication(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	barber, err := storage.CreateBarber(models.Barber{Email: "context_barber@example.com", ShopName: "Context shop", State: "Lagos", LocalArea: "Ikeja"})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: barber.ID, Name: "Context service", Price: 500, Duration: 30, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(barber.ID)
	request := httptest.NewRequest(http.MethodGet, "/explore/"+id+"?state=Lagos&area=Ikeja", nil)
	profile := httptest.NewRecorder()
	ExploreBarberHandler(profile, request, id)
	if profile.Code != 200 {
		t.Fatalf("profile status=%d", profile.Code)
	}
	for _, expected := range []string{"/explore?area=Ikeja&amp;state=Lagos", "Choose this service", "hairstyle%3D" + strconv.Itoa(style.ID)} {
		if !strings.Contains(profile.Body.String(), expected) {
			t.Fatalf("profile lost context %q", expected)
		}
	}
	destination := customerBookingPath(barber.ID, style.ID, url.Values{"state": {"Lagos"}, "area": {"Ikeja"}})
	for _, form := range []url.Values{
		{"email": {"unknown@example.com"}, "password": {"password"}, "next": {destination}},
		{"email": {""}, "password": {""}, "next": {destination}},
	} {
		response := httptest.NewRecorder()
		CustomerLoginHandler(response, authFormRequest("/customer/login", form))
		if response.Code != 401 || !strings.Contains(html.UnescapeString(response.Body.String()), destination) {
			t.Fatalf("failed login lost destination: %d", response.Code)
		}
	}
	rejected := httptest.NewRecorder()
	CustomerRegisterHandler(rejected, authFormRequest("/customer/register", url.Values{"fullname": {"Buyer"}, "phone": {"08012345678"}, "email": {"buyer@example.com"}, "password": {"short"}, "next": {destination}}))
	if rejected.Code != 400 || !strings.Contains(html.UnescapeString(rejected.Body.String()), destination) {
		t.Fatalf("failed signup lost destination: %d", rejected.Code)
	}
	created := httptest.NewRecorder()
	CustomerRegisterHandler(created, discoverySignupRequest(url.Values{"fullname": {"Buyer"}, "phone": {"08012345678"}, "email": {"buyer@example.com"}, "password": {"valid-password"}, "confirm_password": {"valid-password"}, "next": {destination}}))
	if created.Code != 303 || created.Header().Get("Location") != destination {
		t.Fatalf("signup destination: %d %s %s", created.Code, created.Header().Get("Location"), created.Body.String())
	}
	cookies := created.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("signup lacks session cookie")
	}
	authenticated := request.Clone(request.Context())
	authenticated.AddCookie(cookies[0])
	signedIn := httptest.NewRecorder()
	ExploreBarberHandler(signedIn, authenticated, id)
	if signedIn.Code != 200 || strings.Contains(signedIn.Body.String(), ">Customer Login") || !strings.Contains(signedIn.Body.String(), "/customer/dashboard") {
		t.Fatal("public profile ignores signed-in account")
	}
	customer, s, ok := requireCustomer(httptest.NewRecorder(), authenticated)
	if !ok {
		t.Fatal("signup session unusable")
	}
	t.Cleanup(func() { session.Delete(s.ID) })
	detail := httptest.NewRecorder()
	ViewBarberHandler(detail, httptest.NewRequest(http.MethodGet, destination, nil), customer, s, id)
	if detail.Code != 200 || !regexp.MustCompile(`(?s)<input[^>]*name="hairstyle_id"[^>]*value="`+strconv.Itoa(style.ID)+`"[^>]*checked[^>]*>`).MatchString(detail.Body.String()) {
		t.Fatalf("selected service lost: %d", detail.Code)
	}
	login := httptest.NewRecorder()
	CustomerLoginHandler(login, authFormRequest("/customer/login", url.Values{"email": {"buyer@example.com"}, "password": {"valid-password"}, "next": {destination}}))
	if login.Code != 303 || login.Header().Get("Location") != destination {
		t.Fatalf("successful login lost destination: %d %s", login.Code, login.Header().Get("Location"))
	}
	for _, cookie := range login.Result().Cookies() {
		session.Delete(cookie.Value)
	}
	sellerProduct, err := storage.CreateProduct(models.Product{BarberID: barber.ID, Name: "Selected seller product", Quantity: 2, Price: 100, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	otherBarber, err := storage.CreateBarber(models.Barber{Email: "other_context@example.com", ShopName: "Other context shop"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateProduct(models.Product{BarberID: otherBarber.ID, Name: "Other seller product", Quantity: 2, Price: 100, IsAvailable: true}); err != nil {
		t.Fatal(err)
	}
	catalogue := httptest.NewRecorder()
	CustomerProductsHandler(catalogue, httptest.NewRequest(http.MethodGet, "/customer/products?barber="+id, nil), customer, s)
	if catalogue.Code != 200 || !strings.Contains(catalogue.Body.String(), sellerProduct.Name) || strings.Contains(catalogue.Body.String(), "Other seller product") {
		t.Fatal("catalogue lost selected seller")
	}
	initial := httptest.NewRecorder()
	BrowseBarbersHandler(initial, httptest.NewRequest(http.MethodGet, "/customer/barbers", nil), customer, s)
	if strings.Contains(initial.Body.String(), barber.ShopName) || !strings.Contains(initial.Body.String(), "Choose your location") {
		t.Fatal("customer discovery is not location-first")
	}
}

func discoverySignupRequest(values url.Values) *http.Request {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, entries := range values {
		for _, value := range entries {
			writer.WriteField(key, value)
		}
	}
	writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/customer/register", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
