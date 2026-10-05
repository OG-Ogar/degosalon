package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestSellerFulfillmentAndCollectionWording(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	seller, err := storage.CreateBarber(models.Barber{Email: "fulfillment@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := storage.CreateCustomer(models.Customer{Email: "fulfillment_buyer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := storage.CreateProduct(models.Product{BarberID: seller.ID, Name: "Collection product", Price: 1200, Quantity: 2, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = storage.AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	session := sessionData{CSRFToken: "test"}
	cart := httptest.NewRecorder()
	CustomerCartHandler(cart, httptest.NewRequest(http.MethodGet, "/customer/cart", nil), buyer, session)
	if cart.Code != 200 || !strings.Contains(cart.Body.String(), "Place order") || !strings.Contains(cart.Body.String(), "No online payment") || strings.Contains(cart.Body.String(), "Pay ₦") {
		t.Fatalf("collection checkout: %d", cart.Code)
	}
	order, err := storage.CheckoutCart(buyer)
	if err != nil {
		t.Fatal(err)
	}
	items, err := storage.ListProductOrderItems(context.Background(), order.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(items[0].ID)
	action := func(method, token, action string, owner models.Barber) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := authFormRequest("/barber/order-items/"+id+"/"+action, url.Values{"csrf_token": {token}})
		r.Method = method
		BarberOrderItemActionHandler(w, r, owner, session, id, action)
		return w
	}
	if w := action(http.MethodGet, "test", "ready", seller); w.Code != 405 {
		t.Fatalf("GET mutation: %d", w.Code)
	}
	if w := action(http.MethodPost, "wrong", "ready", seller); w.Code != 403 {
		t.Fatalf("CSRF: %d", w.Code)
	}
	other := seller
	other.ID += 999
	if w := action(http.MethodPost, "test", "ready", other); w.Code != 404 {
		t.Fatalf("cross seller: %d", w.Code)
	}
	if w := action(http.MethodPost, "test", "ready", seller); w.Code != 303 {
		t.Fatalf("ready: %d", w.Code)
	}
	page := httptest.NewRecorder()
	CustomerProductOrderConfirmationHandler(page, httptest.NewRequest(http.MethodGet, "/customer/order-confirmation/"+strconv.Itoa(order.ID), nil), buyer, session, strconv.Itoa(order.ID))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Ready") || !strings.Contains(page.Body.String(), "No online payment") {
		t.Fatalf("confirmation: %d", page.Code)
	}
	dashboard := httptest.NewRecorder()
	BarberDashboardHandler(dashboard, httptest.NewRequest(http.MethodGet, "/barber/dashboard", nil), seller, session)
	if dashboard.Code != 200 || !strings.Contains(dashboard.Body.String(), "Mark collected") {
		t.Fatalf("seller actions: %d", dashboard.Code)
	}
	if w := action(http.MethodPost, "test", "collect", seller); w.Code != 303 {
		t.Fatalf("collect: %d", w.Code)
	}
	if w := action(http.MethodPost, "test", "cancel", seller); w.Code != 409 {
		t.Fatalf("closed recovery: %d", w.Code)
	}
}
