package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// ownedHairstyle resolves the {id} segment of a /barber/hairstyles/ URL
// and confirms the signed-in barber owns it, writing the response itself
// when either check fails.
//
// Every hairstyle handler goes through this. The previous code trusted
// the ID in the URL, so any barber could edit or delete another barber's
// hairstyle by changing the number.
func ownedHairstyle(
	w http.ResponseWriter,
	barber models.Barber,
	idSegment string,
) (models.Hairstyle, bool) {
	id, ok := parseID(idSegment)
	if !ok {
		http.Error(w, "Hairstyle not found.", http.StatusNotFound)

		return models.Hairstyle{}, false
	}

	hairstyle, err := storage.FindHairstyleOwnedBy(id, barber.ID)

	switch {
	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		// A barber has no business knowing whether an ID they do not own
		// exists, so both cases answer the same way.
		http.Error(w, "Hairstyle not found.", http.StatusNotFound)

		return models.Hairstyle{}, false

	case err != nil:
		serverError(w, "find hairstyle", err)

		return models.Hairstyle{}, false
	}

	return hairstyle, true
}

// ListHairstylesHandler shows everything the signed-in barber offers.
func ListHairstylesHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	render(w, "barber/hairstyle-list.html", listView[models.Hairstyle]{
		Items: storage.ListHairstylesByBarber(barber.ID),
		CSRF:  s.CSRFToken,
	})
}

// ViewHairstyleHandler shows one hairstyle in full.
func ViewHairstyleHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	hairstyle, ok := ownedHairstyle(w, barber, idSegment)
	if !ok {
		return
	}

	render(w, "barber/hairstyle-profile.html", hairstyleView{
		Hairstyle: hairstyle,
		CSRF:      s.CSRFToken,
	})
}
