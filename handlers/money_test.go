package handlers

import (
	"barber-booking/internal/money"
	"barber-booking/models"
	"barber-booking/storage"
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPriceFormsRejectExcessPrecisionAndBounds(t *testing.T) {
	for _, price := range []string{"0.29", "9999999999.99", "1.001", "NaN", "Inf", "10000000000", "1e2"} {
		values := url.Values{"name": {"Item"}, "category": {"Category"}, "description": {"Description"}, "price": {price}, "quantity": {"1"}, "duration": {"30"}}
		_, productProblem := readProductForm(authFormRequest("/", values))
		_, serviceProblem := readHairstyleForm(authFormRequest("/", values))
		valid := price == "0.29" || price == "9999999999.99"
		if (productProblem == "") != valid || (serviceProblem == "") != valid {
			t.Fatalf("price %q: product=%q service=%q", price, productProblem, serviceProblem)
		}
	}
}

func TestOversizedCartRemainsEditable(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	seller, err := storage.CreateBarber(models.Barber{Email: "cart-seller@example.com", Status: models.BarberActive})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := storage.CreateCustomer(models.Customer{Email: "cart-buyer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := storage.CreateProduct(models.Product{BarberID: seller.ID, Name: "Large item", Price: money.Max.Float(), Quantity: 2, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	if err := storage.SetCartQuantity(context.Background(), buyer.ID, product.ID, 2); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	CustomerCartHandler(response, httptest.NewRequest("GET", "/customer/cart", nil), buyer, sessionData{CSRFToken: "csrf-token"})
	if response.Code != 200 {
		t.Fatalf("cart status %d: %s", response.Code, response.Body.String())
	}
	for _, text := range []string{"Unavailable", "disabled", "Reduce cart amount to place order", "Remove", "csrf-token"} {
		if !strings.Contains(response.Body.String(), text) {
			t.Errorf("missing %q", text)
		}
	}
}
