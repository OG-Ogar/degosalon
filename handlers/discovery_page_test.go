package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestDiscoveryPagingAndReturns(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	var cursor int
	for i := 0; i < 25; i++ {
		b, err := storage.CreateBarber(models.Barber{Email: fmt.Sprintf("paging-%d@example.com", i), ShopName: fmt.Sprintf("Paged Shop %d", i), State: "Lagos", LocalArea: "Ikeja"})
		if err != nil {
			t.Fatal(err)
		}
		if i == 23 {
			cursor = b.ID
		}
	}
	for _, base := range []string{"/explore", "/customer/barbers"} {
		handler := ExploreBarbersHandler
		if base != "/explore" {
			handler = func(w http.ResponseWriter, r *http.Request) {
				BrowseBarbersHandler(w, r, models.Customer{ID: 1}, sessionData{CSRFToken: "token"})
			}
		}
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, base+"?state=Lagos&area=Ikeja", nil))
		body := rec.Body.String()
		if rec.Code != 200 || !strings.Contains(body, "25 active shops") || !strings.Contains(body, "Next shops") || strings.Contains(body, "Previous shops") {
			t.Fatalf("first page %s %d", base, rec.Code)
		}
		next := base + "?state=Lagos&area=Ikeja&after=" + strconv.Itoa(cursor)
		rec = httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, next, nil))
		body = rec.Body.String()
		if rec.Code != 200 || !strings.Contains(body, "Previous shops") || strings.Contains(body, "Next shops") {
			t.Fatal("incorrect next page", base)
		}
		if !strings.Contains(body, "after="+strconv.Itoa(cursor)+"&amp;area=Ikeja&amp;state=Lagos") {
			t.Fatal("card/context lost cursor", base)
		}
		rec = httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, base+"?state=Lagos&area=Ikeja&after=999999", nil))
		if rec.Code != 303 || !strings.Contains(rec.Header().Get("Location"), "area=Ikeja&state=Lagos") {
			t.Fatal("stale page did not recover")
		}
		for _, query := range []string{"after=-1", "after=1&before=2", "after=abc", "after=1&after=2"} {
			rec = httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, base+"?state=Lagos&"+query, nil))
			if rec.Code != 400 {
				t.Fatalf("invalid cursor accepted %s", query)
			}
		}
	}
	next := customerBookingPath(25, 3, url.Values{"state": {"Lagos"}, "area": {"Ikeja"}, "after": {strconv.Itoa(cursor)}})
	if safeCustomerNext(next) != next {
		t.Fatal("auth return lost pagination", next, safeCustomerNext(next))
	}
	rec := httptest.NewRecorder()
	ExploreBarberHandler(rec, httptest.NewRequest(http.MethodGet, "/explore/25?state=Lagos&area=Ikeja&after="+strconv.Itoa(cursor), nil), "25")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "/explore?after="+strconv.Itoa(cursor)+"&amp;area=Ikeja&amp;state=Lagos") {
		t.Fatal("profile return lost current page")
	}
}
