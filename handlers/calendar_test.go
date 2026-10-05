package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestCalendarFormsAndAvailableTimeSelection(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	barber, err := storage.CreateBarber(models.Barber{Email: "calendar_form@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{Email: "calendar_customer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: barber.ID, Name: "Scheduled service", Price: 1000, Duration: 60, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	s := sessionData{CSRFToken: "test"}
	calendar := httptest.NewRecorder()
	BarberCalendarHandler(calendar, httptest.NewRequest(http.MethodGet, "/barber/schedule", nil), barber, s)
	if calendar.Code != 200 || !strings.Contains(calendar.Body.String(), "Minimum notice") {
		t.Fatalf("calendar form: %d", calendar.Code)
	}
	form := url.Values{"csrf_token": {"test"}, "timezone": {"Africa/Lagos"}, "lead": {"30"}, "horizon": {"60"}}
	for i := 0; i < 7; i++ {
		prefix := "day_" + strconv.Itoa(i) + "_"
		form.Set(prefix+"open", "09:00")
		form.Set(prefix+"close", "17:00")
		form.Set(prefix+"break_start", "12:00")
		form.Set(prefix+"break_end", "13:00")
	}
	saved := httptest.NewRecorder()
	BarberCalendarHandler(saved, authFormRequest("/barber/schedule", form), barber, s)
	if saved.Code != 303 {
		t.Fatalf("save calendar: %d %s", saved.Code, saved.Body.String())
	}
	stored, err := storage.CalendarForBarber(context.Background(), barber.ID)
	if err != nil || stored.Week[0].Open != "09:00" {
		t.Fatalf("calendar not saved: %+v %v", stored, err)
	}
	date := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	path := "/customer/barbers/" + strconv.Itoa(barber.ID) + "?hairstyle=" + strconv.Itoa(style.ID) + "&date=" + date
	profile := httptest.NewRecorder()
	ViewBarberHandler(profile, httptest.NewRequest(http.MethodGet, path, nil), customer, s, strconv.Itoa(barber.ID))
	body := profile.Body.String()
	if profile.Code != 200 || !strings.Contains(body, "Show available times") || !strings.Contains(body, `<select id="time"`) || !strings.Contains(body, `value="09:00"`) || strings.Contains(body, `<option value="12:00"`) {
		t.Fatalf("available time form: %d", profile.Code)
	}
	badRequest := httptest.NewRequest(http.MethodGet, "/customer/barbers/"+strconv.Itoa(barber.ID)+"?date=bad", nil)
	malformed := httptest.NewRecorder()
	ViewBarberHandler(malformed, badRequest, customer, s, strconv.Itoa(barber.ID))
	if malformed.Code != 400 {
		t.Fatalf("malformed calendar date: %d", malformed.Code)
	}
	form.Set("csrf_token", "wrong")
	denied := httptest.NewRecorder()
	BarberCalendarHandler(denied, authFormRequest("/barber/schedule", form), barber, s)
	if denied.Code != 403 {
		t.Fatal("calendar mutation lacks CSRF protection")
	}
}
