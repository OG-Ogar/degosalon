package handlers

import (
	"errors"
	"net/http"

	"barber-booking/session"
)

// homeView tells the landing page which "continue as" links to show.
type homeView struct {
	BarberSignedIn   bool
	CustomerSignedIn bool
	AdminSignedIn    bool

	LandingImage string
}

// HomeHandler serves the landing page.
//
// It is registered on "/" and therefore receives every path that the
// other routers do not claim, so anything other than "/" is a 404.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)

		return
	}

	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	_, barberErr := session.FromRequestContext(r, session.RoleBarber)
	if barberErr != nil && !errors.Is(barberErr, session.ErrNotFound) {
		serverError(w, "load home session", barberErr)
		return
	}
	barberIn := barberErr == nil
	_, customerErr := session.FromRequestContext(r, session.RoleCustomer)
	if customerErr != nil && !errors.Is(customerErr, session.ErrNotFound) {
		serverError(w, "load home session", customerErr)
		return
	}
	customerIn := customerErr == nil
	_, adminErr := session.FromRequestContext(r, session.RoleAdmin)
	if adminErr != nil && !errors.Is(adminErr, session.ErrNotFound) {
		serverError(w, "load home session", adminErr)
		return
	}
	adminIn := adminErr == nil
	landingImage, err := currentLandingImage(r.Context())
	if err != nil {
		serverError(w, "load home image setting", err)
		return
	}

	render(w, "home.html", homeView{
		BarberSignedIn:   barberIn,
		CustomerSignedIn: customerIn,
		AdminSignedIn:    adminIn,
		LandingImage:     landingImage,
	})
}
