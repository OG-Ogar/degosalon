package handlers

import (
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberCard is one barber in the customer's browse list, with the
// number of styles they offer so an empty shop is obvious up front.
type barberCard struct {
	models.Barber

	Hairstyles int
	FromPrice  float64
}

// BrowseBarbersHandler lists every active barber on the app.
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

	barbers := storage.ListActiveBarbers()
	cards := make([]barberCard, 0, len(barbers))

	for _, barber := range barbers {
		barber.PasswordHash = ""

		card := barberCard{Barber: barber}

		for _, hairstyle := range storage.ListAvailableHairstylesByBarber(barber.ID) {
			card.Hairstyles++

			if card.Hairstyles == 1 || hairstyle.Price < card.FromPrice {
				card.FromPrice = hairstyle.Price
			}
		}

		cards = append(cards, card)
	}

	render(w, "customer/barber-list.html", listView[barberCard]{
		Items: cards,
		CSRF:  s.CSRFToken,
	})
}

// barberDetailView is one barber's shop as a customer sees it: their
// details, what they offer, and the form to book it.
type barberDetailView struct {
	Barber     models.Barber
	Hairstyles []models.Hairstyle

	// Selected pre-picks a style when the customer arrived from a link.
	Selected int

	Today   string
	Opening string
	Closing string

	ChatLink string
	CSRF     string
	Error    string
	Values   map[string]string
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

	barber, ok := bookableBarber(w, idSegment)
	if !ok {
		return
	}

	selected, _ := parseID(r.URL.Query().Get("hairstyle"))

	renderBarberDetail(w, s, barber, selected, nil, "")
}

// bookableBarber resolves a barber a customer is allowed to see. A
// suspended barber is reported as missing rather than shown, so nobody
// can book with an account the admin has closed.
func bookableBarber(w http.ResponseWriter, idSegment string) (models.Barber, bool) {
	id, ok := parseID(idSegment)
	if !ok {
		http.Error(w, "Barber not found.", http.StatusNotFound)

		return models.Barber{}, false
	}

	barber, found := storage.FindBarberByID(id)
	if !found || !barber.IsActive() {
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
	s sessionData,
	barber models.Barber,
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

	renderStatus(w, status, "customer/barber-detail.html", barberDetailView{
		Barber:     barber,
		Hairstyles: storage.ListAvailableHairstylesByBarber(barber.ID),
		Selected:   selected,
		Today:      today(),
		Opening:    formatHour(openingHour),
		Closing:    formatHour(closingHour),
		ChatLink:   chatPath("/customer/chats/", barber.ID),
		CSRF:       s.CSRFToken,
		Error:      problem,
		Values:     values,
	})
}
