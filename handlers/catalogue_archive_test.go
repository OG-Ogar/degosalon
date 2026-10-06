package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"barber-booking/internal/testdb"
	"barber-booking/models"
	"barber-booking/storage"
)

func TestCatalogueArchiveHTTP(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	checkCatalogueArchiveHTTP(t)
}
func TestPostgresCatalogueArchiveHTTP(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", address)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.CloseDatabase() })
	checkCatalogueArchiveHTTP(t)
}
func checkCatalogueArchiveHTTP(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	previous := imageStores
	imageStores = []imageStore{{dir: root, urlPath: "/uploads/products/"}, {dir: root, urlPath: "/uploads/hairstyles/"}}
	t.Cleanup(func() { imageStores = previous })
	const filename = "1234567890123.png"
	path := filepath.Join(root, filename)
	if err := os.WriteFile(path, []byte("retained original image"), 0600); err != nil {
		t.Fatal(err)
	}
	seller, err := storage.CreateBarber(models.Barber{Email: "archive_seller@example.com", FullName: "Archive seller"})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := storage.CreateCustomer(models.Customer{Email: "archive_buyer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := storage.CreateProduct(models.Product{BarberID: seller.ID, Name: "Archived product", Price: 1250, Quantity: 3, IsAvailable: true, Image: "/uploads/products/" + filename})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: seller.ID, Name: "Archived style", Price: 1000, Duration: 30, IsAvailable: true, Image: "/uploads/hairstyles/" + filename})
	if err != nil {
		t.Fatal(err)
	}
	booking, err := storage.CreateBooking(models.Booking{BarberID: seller.ID, HairstyleID: style.ID, CustomerID: buyer.ID, Date: time.Now().AddDate(0, 0, 7).Format("2006-01-02"), Time: "10:00"})
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
	if _, err := storage.AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	s := sessionData{CSRFToken: "test"}
	for _, operation := range []struct {
		name    string
		id      int
		handler func(http.ResponseWriter, *http.Request, models.Barber, sessionData, string)
	}{{"product", product.ID, DeleteProductHandler}, {"style", style.ID, DeleteHairstyleHandler}} {
		id := strconv.Itoa(operation.id)
		for _, attempt := range []struct {
			method, token string
			owner         models.Barber
			status        int
		}{{http.MethodGet, "test", seller, 405}, {http.MethodPost, "wrong", seller, 403}, {http.MethodPost, "test", models.Barber{ID: seller.ID + 9999}, 404}, {http.MethodPost, "test", seller, 303}, {http.MethodPost, "test", seller, 404}} {
			w := httptest.NewRecorder()
			r := authFormRequest("/archive", url.Values{"csrf_token": {attempt.token}})
			r.Method = attempt.method
			operation.handler(w, r, attempt.owner, s, id)
			if w.Code != attempt.status {
				t.Fatalf("%s archive %s status %d want %d: %s", operation.name, attempt.method, w.Code, attempt.status, w.Body.String())
			}
		}
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "retained original image" {
		t.Fatal("archive deleted original file", err)
	}
	page := httptest.NewRecorder()
	CustomerCartHandler(page, httptest.NewRequest(http.MethodGet, "/customer/cart", nil), buyer, s)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "This product has been archived") || !strings.Contains(page.Body.String(), "/customer/cart/"+strconv.Itoa(product.ID)+"/update") {
		t.Fatalf("archived cart cannot recover: %d %s", page.Code, page.Body.String())
	}
	removed := httptest.NewRecorder()
	UpdateCartItemHandler(removed, authFormRequest("/customer/cart/update", url.Values{"csrf_token": {"test"}, "remove": {"yes"}, "quantity": {"bad"}}), buyer, s, strconv.Itoa(product.ID))
	if removed.Code != 303 {
		t.Fatalf("archive cart removal failed: %d", removed.Code)
	}
	if count, err := storage.CartItemCount(context.Background(), buyer.ID); err != nil || count != 0 {
		t.Fatal("archive cart removal lost", err)
	}
	confirmation := httptest.NewRecorder()
	id := strconv.Itoa(order.ID)
	CustomerProductOrderConfirmationHandler(confirmation, httptest.NewRequest(http.MethodGet, "/customer/order-confirmation/"+id, nil), buyer, s, id)
	if confirmation.Code != 200 || !strings.Contains(confirmation.Body.String(), product.Name) {
		t.Fatal("archival lost order confirmation")
	}
	appointment := httptest.NewRecorder()
	id = strconv.Itoa(booking.ID)
	BookingConfirmationHandler(appointment, httptest.NewRequest(http.MethodGet, "/customer/booking-confirmation/"+id, nil), buyer, s, id)
	if appointment.Code != 200 || !strings.Contains(appointment.Body.String(), style.Name) {
		t.Fatal("archival lost booking confirmation")
	}
}
