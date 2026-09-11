package handlers

import (
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

	_, barberIn := session.FromRequest(r, session.RoleBarber)
	_, customerIn := session.FromRequest(r, session.RoleCustomer)
	_, adminIn := session.FromRequest(r, session.RoleAdmin)

	render(w, "home.html", homeView{
		BarberSignedIn:   barberIn,
		CustomerSignedIn: customerIn,
		AdminSignedIn:    adminIn,
		LandingImage:     currentLandingImage(),
	})
}
