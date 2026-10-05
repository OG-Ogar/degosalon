package handlers

import (
	"errors"
	"net/http"
	"strings"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberCard is one barber in the customer's browse list, with the
// number of styles they offer so an empty shop is obvious up front.
type barberCard struct {
	models.Barber

	Hairstyles  int
	FromPrice   float64
	Rating      float64
	ReviewCount int
}

type browseBarbersView struct {
	Items         []barberCard
	States        []string
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

	barbers, err := storage.ListActiveBarbersResult(r.Context())
	if err != nil {
		serverError(w, "load barber discovery", err)
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	area := strings.TrimSpace(r.URL.Query().Get("area"))
	var states []string
	seen := map[string]bool{}
	for _, b := range barbers {
		if b.State != "" && !seen[strings.ToLower(b.State)] {
			states = append(states, b.State)
			seen[strings.ToLower(b.State)] = true
		}
	}
	cards := make([]barberCard, 0, len(barbers))

	for _, barber := range barbers {
		// Show all active barbers until the customer chooses a location.
		// Once selected, state and area narrow the marketplace results.
		if state != "" && !strings.EqualFold(barber.State, state) {
			continue
		}
		if area != "" && !strings.EqualFold(barber.LocalArea, area) {
			continue
		}
		barber.PasswordHash = ""

		card := barberCard{Barber: barber}

		styles, err := storage.ListAvailableHairstylesByBarberResult(r.Context(), barber.ID)
		if err != nil {
			serverError(w, "load discovery services", err)
			return
		}
		for _, hairstyle := range styles {
			card.Hairstyles++

			if card.Hairstyles == 1 || hairstyle.Price < card.FromPrice {
				card.FromPrice = hairstyle.Price
			}
		}

		card.Rating, card.ReviewCount, err = storage.ReviewSummaryResult(r.Context(), barber.ID)
		if err != nil {
			serverError(w, "load discovery rating", err)
			return
		}

		cards = append(cards, card)
	}

	render(w, "customer/barber-list.html", browseBarbersView{Items: cards, States: states, SelectedState: state, SelectedArea: area, CSRF: s.CSRFToken})
}

// barberDetailView is one barber's shop as a customer sees it: their
// details, what they offer, and the form to book it.
type barberDetailView struct {
	Barber     models.Barber
	Hairstyles []models.Hairstyle
	Products   []models.Product

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
		http.Error(w, "Barber not found.", http.StatusNotFound)

		return models.Barber{}, false
	}

	barber, err := storage.FindBarberByIDContext(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return models.Barber{}, false
	}
	if err != nil {
		serverError(w, "load bookable barber", err)
		return models.Barber{}, false
	}
	if !barber.IsActive() {
		http.Error(w, "Barber not found.", http.StatusNotFound)

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

	products, err := availableProductsByBarber(r, barber.ID)
	if err != nil {
		serverError(w, "load portfolio products", err)
		return
	}
	rating, count, err := storage.ReviewSummaryResult(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load portfolio rating", err)
		return
	}
	styles, err := storage.ListAvailableHairstylesByBarberResult(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load portfolio services", err)
		return
	}
	reviews, err := storage.ListReviewsByBarberResult(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load portfolio reviews", err)
		return
	}
	_, reviewErr := storage.CanCustomerReviewResult(r.Context(), customerID, barber.ID)
	if reviewErr != nil && !errors.Is(reviewErr, storage.ErrNotFound) {
		serverError(w, "load review eligibility", reviewErr)
		return
	}
	canReview := reviewErr == nil
	renderStatus(w, status, "customer/barber-detail.html", barberDetailView{
		Barber:      barber,
		Hairstyles:  styles,
		Products:    products,
		Selected:    selected,
		Today:       today(),
		Opening:     formatHour(openingHour),
		Closing:     formatHour(closingHour),
		ChatLink:    chatPath("/customer/chats/", barber.ID),
		CSRF:        s.CSRFToken,
		Error:       problem,
		Values:      values,
		Reviews:     reviews,
		Rating:      rating,
		ReviewCount: count,
		CanReview:   canReview,
	})
}

func availableProductsByBarber(r *http.Request, barberID int) ([]models.Product, error) {
	var out []models.Product
	products, err := storage.ListProductsByBarber(r.Context(), barberID)
	if err != nil {
		return nil, err
	}
	for _, product := range products {
		if product.IsAvailable && product.Quantity > 0 {
			out = append(out, product)
		}
	}
	return out, nil
}
