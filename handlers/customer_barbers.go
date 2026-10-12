package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberCard is one barber in the customer's browse list, with the
// number of styles they offer so an empty shop is obvious up front.
type barberCard = storage.DiscoveryBarber

type browseBarbersView struct {
	Total         int
	Pagination    discoveryPagination
	ResultsLink   string
	Items         []barberCard
	SelectedState string
	SelectedArea  string
	CSRF          string
}

// BrowseBarbersHandler lists active barbers after a customer chooses a location.
//
// Suspended and deleted barbers are filtered out by storage, and the
// password hash never reaches the view.
func BrowseBarbersHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	page, state, area, pagination, ok := readDiscoveryPage(w, r, "/customer/barbers")
	if !ok {
		return
	}

	render(w, "customer/barber-list.html", browseBarbersView{Items: page.Items, Total: page.Total, Pagination: pagination, ResultsLink: discoveryCardPath("/customer/barbers", 0, state, area, pagination.After, pagination.Before), SelectedState: state, SelectedArea: area, CSRF: s.CSRFToken})
}

// barberDetailView is one barber's shop as a customer sees it: their
// details, what they offer, and the form to book it.
type barberDetailView struct {
	ServicePagination, ProductPagination, ReviewPagination listPagination
	Slots                                                  []string
	Timezone                                               string
	DateMax                                                string
	ResultsLink                                            string
	SelectedState                                          string
	SelectedArea                                           string
	SelectedAfter                                          string
	SelectedBefore                                         string
	Barber                                                 models.Barber
	Hairstyles                                             []models.Hairstyle
	Products                                               []models.Product

	// Selected pre-picks a style when the customer arrived from a link.
	Selected int

	Today   string
	Opening string
	Closing string

	ChatLink    string
	CSRF        string
	Error       string
	Values      map[string]string
	Reviews     []models.Review
	Rating      float64
	ReviewCount int
	CanReview   bool
}

// ViewBarberHandler shows one barber's styles and the booking form.
func ViewBarberHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	barber, ok := bookableBarber(w, r, idSegment)
	if !ok {
		return
	}

	selected, _ := parseID(r.URL.Query().Get("hairstyle"))

	renderBarberDetail(w, r, s, barber, customer.ID, selected, nil, "")
}

// bookableBarber resolves a barber a customer is allowed to see. A
// suspended barber is reported as missing rather than shown, so nobody
// can book with an account the admin has closed.
func bookableBarber(w http.ResponseWriter, r *http.Request, idSegment string) (models.Barber, bool) {
	id, ok := parseID(idSegment)
	if !ok {
		journeyError(w, "Barber not found.", http.StatusNotFound)

		return models.Barber{}, false
	}

	barber, err := storage.FindBarberByIDContext(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return models.Barber{}, false
	}
	if err != nil {
		serverError(w, "load bookable barber", err)
		return models.Barber{}, false
	}
	if !barber.IsActive() {
		journeyError(w, "Barber not found.", http.StatusNotFound)

		return models.Barber{}, false
	}

	barber.PasswordHash = ""

	return barber, true
}

// renderBarberDetail draws the shop page, optionally with a rejected
// booking form filled back in.
func renderBarberDetail(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	barber models.Barber,
	customerID int,
	selected int,
	values map[string]string,
	problem string,
) {
	status := http.StatusOK
	if problem != "" {
		status = http.StatusBadRequest
	}

	if values == nil {
		values = map[string]string{}
	}

	for _, key := range []string{"services_after", "services_before"} {
		values[key] = r.URL.Query().Get(key)
	}
	portfolio, ok := readPortfolio(w, r, barber.ID)
	if !ok {
		return
	}
	styles, products, reviews := portfolio.Services.Items, portfolio.Products.Items, portfolio.Reviews.Items
	rating, count := portfolio.Rating, portfolio.ReviewCount
	if selected == 0 && len(styles) > 0 {
		selected = styles[0].ID
	}
	if selected > 0 {
		found := false
		for _, style := range styles {
			if style.ID == selected {
				found = true
			}
		}
		if !found {
			style, err := storage.FindHairstyleOwnedBy(r.Context(), selected, barber.ID)
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrForbidden) || err == nil && (!style.IsAvailable || style.Archived) {
				journeyError(w, "Page not found.", 404)
				return
			}
			if err != nil {
				serverError(w, "load selected service", err)
				return
			}
			styles = append(styles, style)
		}
	}
	calendar, err := storage.CalendarForBarber(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load booking calendar", err)
		return
	}
	location, err := time.LoadLocation(calendar.Timezone)
	if err != nil {
		serverError(w, "load booking timezone", err)
		return
	}
	day := values["date"]
	if day == "" {
		day = r.URL.Query().Get("date")
	}
	if day == "" {
		day = time.Now().In(location).Format("2006-01-02")
	}
	values["date"] = day
	var slots []string
	if selected > 0 {
		slots, _, err = storage.AvailableSlots(r.Context(), barber.ID, selected, day)
		if errors.Is(err, storage.ErrOutsideSchedule) {
			problem = "Choose a valid date to see available times."
			status = http.StatusBadRequest
		} else if err != nil {
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrForbidden) || errors.Is(err, storage.ErrUnavailable) {
				journeyError(w, "Page not found.", http.StatusNotFound)
			} else {
				serverError(w, "load available times", err)
			}
			return
		}
	}
	_, reviewErr := storage.CanCustomerReviewResult(r.Context(), customerID, barber.ID)
	if reviewErr != nil && !errors.Is(reviewErr, storage.ErrNotFound) {
		serverError(w, "load review eligibility", reviewErr)
		return
	}
	canReview := reviewErr == nil
	renderStatus(w, status, "customer/barber-detail.html", barberDetailView{
		Barber:            barber,
		ServicePagination: portfolio.ServiceLinks, ProductPagination: portfolio.ProductLinks, ReviewPagination: portfolio.ReviewLinks,
		ResultsLink:    discoveryPath("/customer/barbers", discoveryQuery(r)),
		SelectedState:  discoveryQuery(r).Get("state"),
		SelectedArea:   discoveryQuery(r).Get("area"),
		SelectedAfter:  discoveryQuery(r).Get("after"),
		SelectedBefore: discoveryQuery(r).Get("before"),
		Hairstyles:     styles,
		Products:       products,
		Selected:       selected,
		Today:          time.Now().In(location).Format("2006-01-02"),
		DateMax:        time.Now().In(location).AddDate(0, 0, calendar.HorizonDays).Format("2006-01-02"),
		Timezone:       calendar.Timezone,
		Slots:          slots,
		Opening:        formatHour(openingHour),
		Closing:        formatHour(closingHour),
		ChatLink:       chatPath("/customer/chats/", barber.ID),
		CSRF:           s.CSRFToken,
		Error:          problem,
		Values:         values,
		Reviews:        reviews,
		Rating:         rating,
		ReviewCount:    count,
		CanReview:      canReview,
	})
}

// slotsResponse represents available time slots for a barber on a specific date and service.
type slotsResponse struct {
	Slots    []string `json:"slots"`
	Date     string   `json:"date"`
	Timezone string   `json:"timezone"`
	Message  string   `json:"message,omitempty"`
}

// BarberSlotsHandler returns available booking time slots as JSON for dynamic client updates.
func BarberSlotsHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	barber, ok := bookableBarber(w, r, idSegment)
	if !ok {
		return
	}

	styleID, _ := parseID(r.URL.Query().Get("hairstyle"))
	if styleID == 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(slotsResponse{
			Slots:   []string{},
			Message: "A hairstyle is required to check available times.",
		})
		return
	}

	calendar, err := storage.CalendarForBarber(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load booking calendar", err)
		return
	}
	location, err := time.LoadLocation(calendar.Timezone)
	if err != nil {
		serverError(w, "load booking timezone", err)
		return
	}

	day := trimmedField(r.URL.Query().Get("date"))
	if day == "" {
		day = time.Now().In(location).Format("2006-01-02")
	}

	slots, _, err := storage.AvailableSlots(r.Context(), barber.ID, styleID, day)
	if errors.Is(err, storage.ErrOutsideSchedule) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(slotsResponse{
			Slots:    []string{},
			Date:     day,
			Timezone: calendar.Timezone,
			Message:  "No start times are available for this date. Choose another date.",
		})
		return
	}
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrForbidden) || errors.Is(err, storage.ErrUnavailable) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(slotsResponse{
			Slots:   []string{},
			Date:     day,
			Timezone: calendar.Timezone,
			Message:  "That hairstyle is not available.",
		})
		return
	}
	if err != nil {
		serverError(w, "load available slots", err)
		return
	}

	if slots == nil {
		slots = []string{}
	}

	msg := ""
	if len(slots) == 0 {
		msg = "No start times are available for this date. Choose another date."
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(slotsResponse{
		Slots:    slots,
		Date:     day,
		Timezone: calendar.Timezone,
		Message:  msg,
	})
}
