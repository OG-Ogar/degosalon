package handlers

import (
	"context"
	"encoding/json"
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

func TestBarberSlotsEndpoint(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	barber, err := storage.CreateBarber(models.Barber{Email: "slots_test@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{Email: "slots_customer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	style, err := storage.CreateHairstyle(models.Hairstyle{
		BarberID:    barber.ID,
		Name:        "Slots Test Haircut",
		Price:       1500,
		Duration:    30,
		IsAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := sessionData{CSRFToken: "test"}

	// Save calendar
	calendar := models.Calendar{
		LeadMinutes: 30,
		HorizonDays: 30,
		Timezone:    "Africa/Lagos",
	}
	for i := range calendar.Week {
		calendar.Week[i] = models.DayHours{Open: "09:00", Close: "17:00", BreakStart: "12:00", BreakEnd: "13:00"}
	}
	if err := storage.SaveBarberCalendar(context.Background(), barber.ID, calendar); err != nil {
		t.Fatal(err)
	}

	date := time.Now().AddDate(0, 0, 3).Format("2006-01-02")

	// 1. Successful query
	req := httptest.NewRequest(http.MethodGet, "/customer/barbers/"+strconv.Itoa(barber.ID)+"/slots?hairstyle="+strconv.Itoa(style.ID)+"&date="+date, nil)
	rec := httptest.NewRecorder()
	BarberSlotsHandler(rec, req, customer, s, strconv.Itoa(barber.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res slotsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if len(res.Slots) == 0 {
		t.Fatalf("expected non-empty slots, got %+v", res)
	}
	if res.Date != date {
		t.Fatalf("expected date %s, got %s", date, res.Date)
	}
	if res.Timezone != "Africa/Lagos" {
		t.Fatalf("expected timezone Africa/Lagos, got %s", res.Timezone)
	}

	// 2. Missing hairstyle -> 400
	reqMissingStyle := httptest.NewRequest(http.MethodGet, "/customer/barbers/"+strconv.Itoa(barber.ID)+"/slots?date="+date, nil)
	recMissingStyle := httptest.NewRecorder()
	BarberSlotsHandler(recMissingStyle, reqMissingStyle, customer, s, strconv.Itoa(barber.ID))
	if recMissingStyle.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing hairstyle, got %d", recMissingStyle.Code)
	}

	// 3. Date outside schedule -> 200 with empty slots and message
	reqFar := httptest.NewRequest(http.MethodGet, "/customer/barbers/"+strconv.Itoa(barber.ID)+"/slots?hairstyle="+strconv.Itoa(style.ID)+"&date=2099-01-01", nil)
	recFar := httptest.NewRecorder()
	BarberSlotsHandler(recFar, reqFar, customer, s, strconv.Itoa(barber.ID))
	if recFar.Code != http.StatusOK {
		t.Fatalf("expected 200 for outside schedule, got %d", recFar.Code)
	}
	var resFar slotsResponse
	_ = json.Unmarshal(recFar.Body.Bytes(), &resFar)
	if len(resFar.Slots) != 0 || resFar.Message == "" {
		t.Fatalf("expected empty slots with message, got %+v", resFar)
	}

	// 4. Non-GET -> 405
	reqPost := httptest.NewRequest(http.MethodPost, "/customer/barbers/"+strconv.Itoa(barber.ID)+"/slots", nil)
	recPost := httptest.NewRecorder()
	BarberSlotsHandler(recPost, reqPost, customer, s, strconv.Itoa(barber.ID))
	if recPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", recPost.Code)
	}
}

