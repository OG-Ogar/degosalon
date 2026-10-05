package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestCartQuantityControlsAndRecovery(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer, err := storage.CreateCustomer(models.Customer{Email: "edit@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := storage.CreateCustomer(models.Customer{Email: "other@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	barber, err := storage.CreateBarber(models.Barber{Email: "seller@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := storage.CreateProduct(models.Product{BarberID: barber.ID, Name: "Care kit", Price: 500, Quantity: 3, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.AddToCart(customer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	s := sessionData{CSRFToken: "test"}
	id := strconv.Itoa(product.ID)
	page := httptest.NewRecorder()
	CustomerCartHandler(page, httptest.NewRequest(http.MethodGet, "/customer/cart", nil), customer, s)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "/customer/cart/"+id+"/update") || !strings.Contains(page.Body.String(), "Quantity for Care kit") {
		t.Fatalf("cart controls: %d %s", page.Code, page.Body.String())
	}
	for _, tc := range []struct {
		name, quantity, token, remove string
		buyer                         models.Customer
		status, count                 int
	}{
		{"csrf denied", "2", "wrong", "", customer, 403, 1},
		{"invalid quantity", "1.5", "test", "", customer, 400, 1},
		{"other customer", "0", "test", "", other, 404, 1},
		{"stock conflict", "4", "test", "", customer, 409, 1},
		{"update", "2", "test", "", customer, 303, 2},
		{"remove despite invalid input", "bad", "test", "yes", customer, 303, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := authFormRequest("/customer/cart/"+id+"/update", url.Values{"csrf_token": {tc.token}, "quantity": {tc.quantity}, "remove": {tc.remove}})
			UpdateCartItemHandler(response, request, tc.buyer, s, id)
			if response.Code != tc.status {
				t.Fatalf("status %d want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if tc.status == 409 && !strings.Contains(response.Body.String(), "role=\"alert\"") {
				t.Fatal("stock conflict lacks recovery feedback")
			}
			count, err := storage.CartItemCount(context.Background(), customer.ID)
			if err != nil || count != tc.count {
				t.Fatalf("cart count=%d want %d: %v", count, tc.count, err)
			}
		})
	}
	// The actual rendered checkout form supplies a stable retry key.
	matches := regexp.MustCompile(`name="checkout_key" value="([a-f0-9]{32})"`).FindStringSubmatch(page.Body.String())
	if len(matches) != 2 {
		t.Fatal("cart form lacks a checkout retry key")
	}
	if _, err := storage.AddToCart(customer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	invalid := httptest.NewRecorder()
	CustomerProductOrderHandler(invalid, authFormRequest("/customer/products/pay", url.Values{"csrf_token": {"test"}}), customer, s)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("missing key status=%d", invalid.Code)
	}
	location := ""
	for i := 0; i < 3; i++ {
		if i == 2 {
			if _, err := storage.AddToCart(customer.ID, product.ID); err != nil {
				t.Fatal(err)
			}
		}
		response := httptest.NewRecorder()
		request := authFormRequest("/customer/products/pay", url.Values{"csrf_token": {"test"}, "checkout_key": {matches[1]}, "amount": {"0.01"}})
		CustomerProductOrderHandler(response, request, customer, s)
		if response.Code != http.StatusSeeOther {
			t.Fatalf("checkout %d status=%d: %s", i, response.Code, response.Body.String())
		}
		if i == 0 {
			location = response.Header().Get("Location")
		} else if response.Header().Get("Location") != location {
			t.Fatal("replay redirected to a different order")
		}
	}
	orders, err := storage.ListProductOrdersByCustomer(context.Background(), customer.ID)
	if err != nil || len(orders) != 1 || orders[0].TotalAmount != 500 {
		t.Fatalf("checkout totals or replay: %+v %v", orders, err)
	}
	count, err := storage.CartItemCount(context.Background(), customer.ID)
	if err != nil || count != 1 {
		t.Fatalf("replay consumed newer cart: %d %v", count, err)
	}

}
