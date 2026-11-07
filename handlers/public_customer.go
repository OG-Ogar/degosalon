package handlers

import (
	"errors"
	"net/http"
	"strings"

	"barber-booking/storage"
)

// publicCustomerView exposes only what a feed already shows about a customer:
// the display name and the profile photo. Contact details stay private.
type publicCustomerView struct {
	Name  string
	Image string
}

// PublicCustomerProfileHandler serves the small profile page behind a
// customer's avatar in the community feed.
func PublicCustomerProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	segment := strings.TrimPrefix(r.URL.Path, "/people/")
	if segment == "" || strings.Contains(segment, "/") {
		journeyError(w, "Profile not found.", http.StatusNotFound)

		return
	}
	id, ok := parseID(segment)
	if !ok {
		journeyError(w, "Profile not found.", http.StatusNotFound)

		return
	}

	customer, err := storage.FindCustomerByIDResult(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Profile not found.", http.StatusNotFound)

		return
	}
	if err != nil {
		serverError(w, "load public customer profile", err)

		return
	}

	render(w, "public-customer.html", publicCustomerView{Name: customer.FullName, Image: customer.ProfileImage})
}
