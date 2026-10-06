package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// confirmDeleteView backs the "are you sure" page.
type confirmDeleteView struct {
	Title   string
	Message string
	Action  string
	Cancel  string
	CSRF    string
}

// ConfirmDeleteHairstyleHandler asks before removing a hairstyle. The
// delete itself is POST only, so it cannot happen by following a link.
func ConfirmDeleteHairstyleHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet, http.MethodPost)

		return
	}

	hairstyle, ok := ownedHairstyle(w, r, barber, idSegment)
	if !ok {
		return
	}

	render(w, "barber/confirm-delete-hairstyle.html", confirmDeleteView{
		Title:   "Archive hairstyle",
		Message: "Archive " + hairstyle.Name + "? It will leave your catalogue. Existing bookings and media will be retained.",
		Action:  hairstylePath(hairstyle.ID, "delete"),
		Cancel:  "/barber/hairstyles",
		CSRF:    s.CSRFToken,
	})
}

// DeleteHairstyleHandler archives a hairstyle and retains its media.
func DeleteHairstyleHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	id, ok := parseID(idSegment)
	if !ok {
		journeyError(w, "Hairstyle not found.", http.StatusNotFound)

		return
	}

	_, err := storage.DeleteHairstyleContext(r.Context(), id, barber.ID)

	switch {
	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		journeyError(w, "Hairstyle not found.", http.StatusNotFound)

		return

	case err != nil:
		serverError(w, "archive hairstyle", err)

		return
	}

	http.Redirect(w, r, "/barber/hairstyles", http.StatusSeeOther)
}
