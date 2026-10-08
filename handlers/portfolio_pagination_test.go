package handlers

import (
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNamedPaginationKeepsOtherSectionsAndDiscovery(t *testing.T) {
	r := httptest.NewRequest("GET", "/customer/barbers/42?state=Lagos&area=Ikeja&after=41&services_after=24&products_after=80&hairstyle=99", nil)
	cursor, ok := readNamedPageCursor(httptest.NewRecorder(), r, "services")
	if !ok || cursor.After != 24 {
		t.Fatal("discovery cursor used as service cursor")
	}
	links := namedPaginationLinks(r, "services", "booking", cursor, 25, 48, true, true, true)
	for _, raw := range []string{links.NextLink, links.PreviousLink, links.FirstLink} {
		address, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		q := address.Query()
		if q.Get("after") != "41" || q.Get("products_after") != "80" || q.Get("hairstyle") != "99" || address.Fragment != "booking" {
			t.Fatalf("lost context %s", raw)
		}
	}
	first, _ := url.Parse(links.FirstLink)
	if first.Query().Has("services_after") || first.Query().Has("services_before") {
		t.Fatal("first page retains own cursor")
	}
	for _, query := range []string{"services_after=1&services_before=2", "services_after=1&services_after=2", "services_after=bad"} {
		response := httptest.NewRecorder()
		if _, ok := readNamedPageCursor(response, httptest.NewRequest("GET", "/?"+query, nil), "services"); ok || response.Code != 400 {
			t.Fatal("invalid cursor accepted")
		}
	}
}
