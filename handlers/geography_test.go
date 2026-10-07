package handlers

import (
	"barber-booking/internal/geography"
	"barber-booking/models"
	"barber-booking/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCanonicalBarberForm(t *testing.T) {
	s, a, ok := geography.Resolve("Lagos", "Ikeja")
	if !ok {
		t.Fatal("missing Lagos/Ikeja")
	}
	base := url.Values{"fullname": {"Barber Name"}, "email": {"barber@example.com"}, "phone": {"08012345678"}, "shopname": {"Shop"}, "shopaddress": {"12 Street"}, "state": {s.ID}, "local_area": {a.ID}}
	request := func(v url.Values) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return r
	}
	form, problem := readBarberForm(request(base))
	if problem != "" || form.State != "Lagos" || form.LocalArea != "Ikeja" {
		t.Fatalf("%+v %s", form, problem)
	}
	for _, area := range []string{"Abuja Municipal", "Victoria Island", "NG015001"} {
		base.Set("local_area", area)
		if _, problem := readBarberForm(request(base)); problem == "" {
			t.Fatalf("accepted invalid pair %s", area)
		}
	}
}

func TestCanonicalDiscovery(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	if err := LoadTemplates("../templates"); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateBarber(models.Barber{Email: "canonical@example.com", ShopName: "Canonical Shop", State: "lagos state", LocalArea: "Ikeja"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/explore?state=NG025&area=NG025011", "/explore?state=Lagos&area=Ikeja", "/explore?state=FCT"} {
		rec := httptest.NewRecorder()
		ExploreBarbersHandler(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != 200 {
			t.Fatalf("%s=%d %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(path, "FCT") && !strings.Contains(rec.Body.String(), "Canonical Shop") {
			t.Fatal("lost match")
		}
		for _, label := range []string{">Zamfara</option>", ">FCT</option>", "data-geography-area"} {
			if !strings.Contains(rec.Body.String(), label) {
				t.Fatal("missing choice", label)
			}
		}
	}
	for _, path := range []string{"/explore?state=Lagos&area=Abuja+Municipal", "/explore?state=unknown", "/explore?area=Ikeja"} {
		rec := httptest.NewRecorder()
		ExploreBarbersHandler(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != 400 {
			t.Fatalf("invalid query accepted %s", path)
		}
	}
}
