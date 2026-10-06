package handlers

import (
	"net"
	"net/http"
	"os"
	"testing"

	"barber-booking/models"
	"barber-booking/security"
	"barber-booking/storage"
)

// This fixture is compiled only into tests and runs only for Playwright.
// It uses real templates, routing and authentication with synthetic memory data.
func TestBrowserFixtureServer(t *testing.T) {
	if os.Getenv("BROWSER_TEST_SERVER") != "1" {
		t.Skip("Playwright fixture only")
	}
	storage.Reset()
	t.Cleanup(storage.Reset)
	hash, err := security.HashPassword("Browser-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.CreateCustomer(models.Customer{FullName: "Browser Customer", Email: "browser@example.com", PhoneNumber: "08012345678", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	barber, err := storage.CreateBarber(models.Barber{FullName: "Browser Barber", Email: "barber-browser@example.com", PasswordHash: hash, ShopName: "Browser Shop", State: "Lagos", LocalArea: "Ikeja", ProfileImage: "/__browser-image.svg"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.CreateHairstyle(models.Hairstyle{BarberID: barber.ID, Name: "Browser Cut", Price: 1250, Duration: 30, IsAvailable: true, Image: "/__browser-image.svg"})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", HomeHandler)
	mux.HandleFunc("/customer/", CustomerRouter)
	mux.HandleFunc("/barber/", BarberRouter)
	mux.HandleFunc("/explore", ExploreRouter)
	mux.HandleFunc("/explore/", ExploreRouter)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("../static"))))
	mux.HandleFunc("/__browser-image.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="200"><rect width="300" height="200" fill="#254137"/><circle cx="150" cy="100" r="50" fill="#f4cb78"/></svg>`))
	})
	listener, err := net.Listen("tcp", "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux}
	t.Cleanup(func() { server.Close() })
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
