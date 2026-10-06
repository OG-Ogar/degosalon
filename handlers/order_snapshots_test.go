package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"barber-booking/internal/testdb"
	"barber-booking/models"
	"barber-booking/storage"
)

func TestOrderConfirmationSnapshots(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	checkOrderConfirmationSnapshots(t, nil)
}

func TestPostgresOrderConfirmationSnapshots(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", address)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.CloseDatabase() })
	conn, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	checkOrderConfirmationSnapshots(t, conn)
}

func checkOrderConfirmationSnapshots(t *testing.T, conn *sql.DB) {
	t.Helper()
	seller, err := storage.CreateBarber(models.Barber{Email: "snapshot_seller@example.com", FullName: "Original seller", ShopName: "Original shop", ShopAddress: "Original address", PhoneNumber: "08012345678"})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := storage.CreateCustomer(models.Customer{Email: "snapshot_buyer@example.com", FullName: "Buyer"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := storage.CreateProduct(models.Product{BarberID: seller.ID, Name: "Original product", Description: "Original description", Category: "Original category", Price: 1250.50, Quantity: 3, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	order, err := storage.CheckoutCart(buyer)
	if err != nil {
		t.Fatal(err)
	}
	seller.FullName = "Edited seller"
	seller.ShopName = "Edited shop"
	seller.ShopAddress = "Edited address"
	seller.PhoneNumber = "08087654321"
	if err := storage.UpdateBarber(seller); err != nil {
		t.Fatal(err)
	}
	product, err = storage.FindProductByIDResult(context.Background(), product.ID)
	if err != nil {
		t.Fatal(err)
	}
	product.Name = "Edited product"
	product.Description = "Edited description"
	product.Category = "Edited category"
	product.Price = 9999
	if err := storage.UpdateProduct(product); err != nil {
		t.Fatal(err)
	}
	// A renamed live catalogue proves this request does not depend on it at all.
	if conn != nil {
		if _, err := conn.Exec("alter table public.products rename to unavailable_products;alter table public.barbers rename to unavailable_barbers"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := conn.Exec("alter table public.unavailable_products rename to products;alter table public.unavailable_barbers rename to barbers"); err != nil {
				t.Error(err)
			}
		}()
	}
	id := strconv.Itoa(order.ID)
	show := func(customer models.Customer) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		CustomerProductOrderConfirmationHandler(w, httptest.NewRequest(http.MethodGet, "/customer/order-confirmation/"+id, nil), customer, sessionData{CSRFToken: "test"}, id)
		return w
	}
	w := show(buyer)
	if w.Code != 200 {
		t.Fatalf("confirmation status %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"Original seller", "Original shop", "Original address", "08012345678", "Original product", "Original description", "Original category", "1250.50", "Pending"} {
		mustContain(t, "order snapshot", body, want)
	}
	for _, unwanted := range []string{"Edited seller", "Edited shop", "Edited address", "08087654321", "Edited product", "Edited description", "Edited category", "9999.00"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("confirmation displayed live metadata: %s", unwanted)
		}
	}
	other := buyer
	other.ID += 99999
	if w := show(other); w.Code != 404 {
		t.Fatalf("cross customer order leaked: %d", w.Code)
	}
}
