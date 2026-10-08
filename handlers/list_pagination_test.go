package handlers

import (
	"barber-booking/storage"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestListPaginationPreservesFiltersAndRejectsInvalidCursors(t *testing.T) {
	for _, query := range []string{"after=0", "before=-1", "after=1&before=2", "after=1&after=2", "before=nope"} {
		response := httptest.NewRecorder()
		if _, ok := readPageCursor(response, httptest.NewRequest("GET", "/customer/products?"+query, nil)); ok || response.Code != 400 {
			t.Fatalf("invalid cursor accepted: %s", query)
		}
	}
	request := httptest.NewRequest("GET", "/customer/products?barber=42&product=55&after=10", nil)
	links := paginationLinks(request, storage.PageCursor{After: 10}, 11, 34, true, true, true)
	previous, _ := url.Parse(links.PreviousLink)
	next, _ := url.Parse(links.NextLink)
	first, _ := url.Parse(links.FirstLink)
	if previous.Query().Get("before") != "11" || next.Query().Get("after") != "34" || first.Query().Get("after") != "" {
		t.Fatalf("bad page links: %+v", links)
	}
	for _, link := range []*url.URL{previous, next, first} {
		if link.Query().Get("barber") != "42" || link.Query().Get("product") != "55" {
			t.Fatal("catalogue filter lost")
		}
	}
	descending := paginationLinks(request, storage.PageCursor{Before: 35}, 34, 11, false, true, true)
	older, _ := url.Parse(descending.NextLink)
	newer, _ := url.Parse(descending.PreviousLink)
	if older.Query().Get("before") != "11" || newer.Query().Get("after") != "34" {
		t.Fatal("descending cursor directions swapped")
	}
}
