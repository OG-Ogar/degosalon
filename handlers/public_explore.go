package handlers

import (
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type exploreCard struct {
	models.Barber
	Hairstyles  int
	FromPrice   float64
	Rating      float64
	ReviewCount int
}
type exploreView struct {
	Items                       []exploreCard
	States                      []string
	SelectedState, SelectedArea string
}
type publicBarberView struct {
	ResultsLink      string
	LoginLink        string
	RegisterLink     string
	ServiceLinks     map[int]string
	ShopLink         string
	Barber           models.Barber
	Hairstyles       []models.Hairstyle
	Products         []models.Product
	Reviews          []models.Review
	Rating           float64
	ReviewCount      int
	CustomerSignedIn bool
	BookLink         string
	MessageLink      string
}

func ExploreRouter(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path)
	if len(parts) == 1 {
		ExploreBarbersHandler(w, r)
		return
	}
	if len(parts) == 2 {
		ExploreBarberHandler(w, r, parts[1])
		return
	}
	http.NotFound(w, r)
}

func ExploreBarbersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	all, err := storage.ListActiveBarbersResult(r.Context())
	if err != nil {
		serverError(w, "load public discovery", err)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	area := strings.TrimSpace(r.URL.Query().Get("area"))
	seen := map[string]bool{}
	var states []string
	var cards []exploreCard
	for _, b := range all {
		if b.State != "" && !seen[strings.ToLower(b.State)] {
			states = append(states, b.State)
			seen[strings.ToLower(b.State)] = true
		}
		if state == "" || !strings.EqualFold(b.State, state) || (area != "" && !strings.EqualFold(b.LocalArea, area)) {
			continue
		}
		b.PasswordHash = ""
		card := exploreCard{Barber: b}
		styles, err := storage.ListAvailableHairstylesByBarberResult(r.Context(), b.ID)
		if err != nil {
			serverError(w, "load public services", err)
			return
		}
		for _, style := range styles {
			card.Hairstyles++
			if card.Hairstyles == 1 || style.Price < card.FromPrice {
				card.FromPrice = style.Price
			}
		}
		card.Rating, card.ReviewCount, err = storage.ReviewSummaryResult(r.Context(), b.ID)
		if err != nil {
			serverError(w, "load public rating", err)
			return
		}
		cards = append(cards, card)
	}
	sort.Strings(states)
	render(w, "explore.html", exploreView{Items: cards, States: states, SelectedState: state, SelectedArea: area})
}

func ExploreBarberHandler(w http.ResponseWriter, r *http.Request, idSegment string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id, ok := parseID(idSegment)
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := storage.FindBarberByIDContext(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load public barber", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) || !b.IsActive() {
		http.NotFound(w, r)
		return
	}
	b.PasswordHash = ""
	products, err := availableProductsByBarber(r, id)
	if err != nil {
		serverError(w, "load public portfolio products", err)
		return
	}
	rating, count, err := storage.ReviewSummaryResult(r.Context(), id)
	if err != nil {
		serverError(w, "load public portfolio rating", err)
		return
	}
	styles, err := storage.ListAvailableHairstylesByBarberResult(r.Context(), id)
	if err != nil {
		serverError(w, "load public portfolio services", err)
		return
	}
	reviews, err := storage.ListReviewsByBarberResult(r.Context(), id)
	if err != nil {
		serverError(w, "load public portfolio reviews", err)
		return
	}

	signedIn := false
	if s, ok := session.FromRequest(r, session.RoleCustomer); ok {
		_, err := storage.FindCustomerByIDResult(r.Context(), s.UserID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "load public account context", err)
			return
		}
		signedIn = err == nil
	}
	query := discoveryQuery(r)
	bookingPath := customerBookingPath(id, 0, query)
	loginLink := customerLoginPath(bookingPath)
	registerLink := discoveryPath("/customer/register", url.Values{"next": {bookingPath}})
	bookLink := loginLink
	messageLink := customerLoginPath("/customer/chats/" + idSegment)
	shopPath := discoveryPath("/customer/products", url.Values{"barber": {strconv.Itoa(id)}})
	shopLink := customerLoginPath(shopPath)
	if signedIn {
		bookLink = bookingPath
		messageLink = "/customer/chats/" + idSegment
		shopLink = shopPath
	}
	serviceLinks := map[int]string{}
	for _, style := range styles {
		destination := customerBookingPath(id, style.ID, query)
		if !signedIn {
			destination = customerLoginPath(destination)
		}
		serviceLinks[style.ID] = destination
	}
	render(w, "explore-barber.html", publicBarberView{ResultsLink: discoveryPath("/explore", query), LoginLink: loginLink, RegisterLink: registerLink, ServiceLinks: serviceLinks, ShopLink: shopLink, Barber: b, Hairstyles: styles, Products: products, Reviews: reviews, Rating: rating, ReviewCount: count, CustomerSignedIn: signedIn, BookLink: bookLink, MessageLink: messageLink})
}
