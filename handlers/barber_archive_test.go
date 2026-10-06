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

func TestBarberArchiveHTTP(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	checkBarberArchiveHTTP(t)
}
func TestPostgresBarberArchiveHTTP(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", address)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.CloseDatabase() })
	checkBarberArchiveHTTP(t)
}
func checkBarberArchiveHTTP(t *testing.T) {
	if err := LoadTemplates("../templates"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	previous := imageStores
	imageStores = []imageStore{{dir: root, urlPath: "/uploads/barbers/"}}
	t.Cleanup(func() { imageStores = previous })
	path := filepath.Join(root, "1234567890123.png")
	if err := os.WriteFile(path, []byte("retained media"), 0600); err != nil {
		t.Fatal(err)
	}
	seller, err := storage.CreateBarber(models.Barber{Email: "archive_http_seller@example.com", FullName: "Retained seller", ProfileImage: "/uploads/barbers/1234567890123.png"})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := storage.CreateCustomer(models.Customer{Email: "archive_http_buyer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: seller.ID, Name: "Retained cut", Duration: 30, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	booking, err := storage.CreateBooking(models.Booking{BarberID: seller.ID, CustomerID: buyer.ID, HairstyleID: style.ID, Date: time.Now().AddDate(0, 0, 7).Format("2006-01-02"), Time: "10:00"})
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(seller.ID)
	s := sessionData{CSRFToken: "test"}
	for _, attempt := range []struct {
		method, token string
		status        int
	}{{"GET", "test", 405}, {"POST", "wrong", 403}, {"POST", "test", 409}} {
		w := httptest.NewRecorder()
		r := authFormRequest("/admin/barbers/"+id+"/delete", url.Values{"csrf_token": {attempt.token}})
		r.Method = attempt.method
		DeleteBarberHandler(w, r, s, id)
		if w.Code != attempt.status {
			t.Fatalf("archive status: %d want %d: %s", w.Code, attempt.status, w.Body.String())
		}
		if w.Code == 409 && (!strings.Contains(w.Body.String(), "Resolve pending") || !strings.Contains(w.Header().Get("Content-Type"), "text/html")) {
			t.Fatal("conflict lacks recovery page")
		}
	}
	if err := storage.SetBookingStatus(booking.ID, models.StatusCancelled, 0, buyer.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		DeleteBarberHandler(w, authFormRequest("/archive", url.Values{"csrf_token": {"test"}}), s, id)
		if w.Code != http.StatusSeeOther {
			t.Fatalf("archive/retry: %d %s", w.Code, w.Body.String())
		}
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "retained media" {
		t.Fatal("archive deleted media", err)
	}
	stored, err := storage.FindBarberByIDContext(context.Background(), seller.ID)
	if err != nil || stored.Status != models.BarberArchived {
		t.Fatalf("account not archived: %+v %v", stored, err)
	}
	w := httptest.NewRecorder()
	ListBarbersHandler(w, httptest.NewRequest("GET", "/admin/barbers", nil), s)
	if w.Code != 200 || strings.Contains(w.Body.String(), "/admin/barbers/"+id+"/activate") || strings.Contains(w.Body.String(), "/admin/barbers/"+id+"/edit") {
		t.Fatal("archived account exposes mutation links")
	}
	w = httptest.NewRecorder()
	SetBarberStatusHandler(w, authFormRequest("/activate", url.Values{"csrf_token": {"test"}}), s, id, true)
	if w.Code != 404 {
		t.Fatalf("reactivation: %d", w.Code)
	}
}
