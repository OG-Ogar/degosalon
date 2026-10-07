package handlers

import (
	"barber-booking/internal/geography"
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
	"errors"
	"net/http"
	"net/url"
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
	CustomerSignedIn bool
	LoginLink        string

	Items                       []exploreCard
	SelectedState, SelectedArea string
}
type publicBarberView struct {
	ResultsLink      string
	LoginLink        string
	RegisterLink     string
	ServiceLinks     map[int]string
	ProductLinks     map[int]string
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
	journeyError(w, "Page not found.", http.StatusNotFound)
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
	state, area, ok := discoveryLocation(r)
	if !ok {
		journeyError(w, "Choose a valid state/FCT and an LGA or area council within it.", http.StatusBadRequest)
		return
	}

	var cards []exploreCard
	for _, b := range all {
		b.State, b.LocalArea = geography.NormalizeLegacy(b.State, b.LocalArea)
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
	signedIn, sessionErr := publicCustomerSignedIn(r)
	if sessionErr != nil {
		serverError(w, "load discovery session", sessionErr)
		return
	}
	destination := discoveryPath("/customer/barbers", discoveryQuery(r))
	render(w, "explore.html", exploreView{Items: cards, SelectedState: state, SelectedArea: area, CustomerSignedIn: signedIn, LoginLink: customerLoginPath(destination)})
}

func ExploreBarberHandler(w http.ResponseWriter, r *http.Request, idSegment string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id, ok := parseID(idSegment)
	if !ok {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}
	b, err := storage.FindBarberByIDContext(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load public barber", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) || !b.IsActive() {
		journeyError(w, "Page not found.", http.StatusNotFound)
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

	signedIn, sessionErr := publicCustomerSignedIn(r)
	if sessionErr != nil {
		serverError(w, "load public account context", sessionErr)
		return
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
	productLinks := map[int]string{}
	for _, product := range products {
		destination := discoveryPath("/customer/products", url.Values{"barber": {strconv.Itoa(id)}, "product": {strconv.Itoa(product.ID)}})
		if !signedIn {
			destination = customerLoginPath(destination)
		}
		productLinks[product.ID] = destination
	}
	serviceLinks := map[int]string{}
	for _, style := range styles {
		destination := customerBookingPath(id, style.ID, query)
		if !signedIn {
			destination = customerLoginPath(destination)
		}
		serviceLinks[style.ID] = destination
	}
	render(w, "explore-barber.html", publicBarberView{ResultsLink: discoveryPath("/explore", query), LoginLink: loginLink, RegisterLink: registerLink, ServiceLinks: serviceLinks, ProductLinks: productLinks, ShopLink: shopLink, Barber: b, Hairstyles: styles, Products: products, Reviews: reviews, Rating: rating, ReviewCount: count, CustomerSignedIn: signedIn, BookLink: bookLink, MessageLink: messageLink})
}

func publicCustomerSignedIn(r *http.Request) (bool, error) {
	s, err := session.FromRequestContext(r, session.RoleCustomer)
	if errors.Is(err, session.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = storage.FindCustomerByIDResult(r.Context(), s.UserID)
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
