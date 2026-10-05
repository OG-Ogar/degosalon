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
		Title:   "Delete hairstyle",
		Message: "Delete " + hairstyle.Name + "? This cannot be undone.",
		Action:  hairstylePath(hairstyle.ID, "delete"),
		Cancel:  "/barber/hairstyles",
		CSRF:    s.CSRFToken,
	})
}

// DeleteHairstyleHandler removes a hairstyle and its image file.
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
		http.Error(w, "Hairstyle not found.", http.StatusNotFound)

		return
	}

	image, err := storage.DeleteHairstyle(id, barber.ID)

	switch {
	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		http.Error(w, "Hairstyle not found.", http.StatusNotFound)

		return

	case err != nil:
		serverError(w, "delete hairstyle", err)

		return
	}

	// The record is gone, so a failure to unlink the file is worth a log
	// line but not an error page.
	removeImageIgnoringError(image)

	http.Redirect(w, r, "/barber/hairstyles", http.StatusSeeOther)
}
