package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestCustomerDashboardShowsOnlyOwnedOrderHistory(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	customer := models.Customer{ID: 1, FullName: "Signed in customer"}
	for _, order := range []models.ProductOrder{
		{CustomerID: 1, CustomerName: "Owned purchase", TotalAmount: 1250, Status: models.OrderPending, CreatedAt: time.Now()},
		{CustomerID: 2, CustomerName: "Private other purchase", TotalAmount: 9900, Status: models.OrderPaid, CreatedAt: time.Now()},
	} {
		if _, err := storage.CreateProductOrder(order); err != nil {
			t.Fatal(err)
		}
	}
	response := httptest.NewRecorder()
	CustomerDashboardHandler(response, httptest.NewRequest(http.MethodGet, "/customer/dashboard", nil), customer, sessionData{CSRFToken: "test"})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{"Owned purchase", "₦1,250.00", "/customer/order-confirmation/1"} {
		mustContain(t, "dashboard orders", body, want)
	}
	if strings.Contains(body, "Private other purchase") || strings.Contains(body, "/customer/order-confirmation/2") {
		t.Fatal("dashboard exposed another customer's order")
	}
}
