package handlers

import (
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

type exploreCard = storage.DiscoveryBarber
type exploreView struct {
	Total            int
	Pagination       discoveryPagination
	CustomerSignedIn bool
	LoginLink        string

	Items                       []exploreCard
	SelectedState, SelectedArea string
}
type publicBarberView struct {
	ServicePagination, ProductPagination, ReviewPagination listPagination
	ResultsLink                                            string
	LoginLink                                              string
	RegisterLink                                           string
	ServiceLinks                                           map[int]string
	ProductLinks                                           map[int]string
	ShopLink                                               string
	Barber                                                 models.Barber
	Hairstyles                                             []models.Hairstyle
	Products                                               []models.Product
	Reviews                                                []models.Review
	Rating                                                 float64
	ReviewCount                                            int
	CustomerSignedIn                                       bool
	BookLink                                               string
	MessageLink                                            string
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
	page, state, area, pagination, ok := readDiscoveryPage(w, r, "/explore")
	if !ok {
		return
	}

	signedIn, sessionErr := publicCustomerSignedIn(r)
	if sessionErr != nil {
		serverError(w, "load discovery session", sessionErr)
		return
	}
	destination := discoveryPath("/customer/barbers", discoveryQuery(r))
	render(w, "explore.html", exploreView{Items: page.Items, Total: page.Total, Pagination: pagination, SelectedState: state, SelectedArea: area, CustomerSignedIn: signedIn, LoginLink: customerLoginPath(destination)})
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
	portfolio, ok := readPortfolio(w, r, id)
	if !ok {
		return
	}
	styles, products, reviews := portfolio.Services.Items, portfolio.Products.Items, portfolio.Reviews.Items
	rating, count := portfolio.Rating, portfolio.ReviewCount

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
	render(w, "explore-barber.html", publicBarberView{ServicePagination: portfolio.ServiceLinks, ProductPagination: portfolio.ProductLinks, ReviewPagination: portfolio.ReviewLinks, ResultsLink: discoveryPath("/explore", query), LoginLink: loginLink, RegisterLink: registerLink, ServiceLinks: serviceLinks, ProductLinks: productLinks, ShopLink: shopLink, Barber: b, Hairstyles: styles, Products: products, Reviews: reviews, Rating: rating, ReviewCount: count, CustomerSignedIn: signedIn, BookLink: bookLink, MessageLink: messageLink})
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
