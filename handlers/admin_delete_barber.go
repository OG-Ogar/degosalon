package handlers

import (
	"errors"
	"log"
	"net/http"

	"barber-booking/session"
	"barber-booking/storage"
)

// ConfirmDeleteBarberHandler shows the "are you sure" page. Deletion
// itself only happens on POST, so no link or crawler can trigger it.
func ConfirmDeleteBarberHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	id, ok := parseID(idSegment)
	if !ok {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)

		return
	}

	barber, err := storage.FindBarberByIDContext(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load admin barber", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	barber.PasswordHash = ""

	render(w, "admin/confirm-delete.html", barberView{
		Barber: barber,
		CSRF:   s.CSRFToken,
	})
}

// DeleteBarberHandler removes a barber and everything attached to them.
//
// The store performs the cascade under one lock and hands back the image
// paths; the files are deleted afterwards, and a failure to unlink a file
// is logged rather than aborting a delete that has already happened.
func DeleteBarberHandler(
	w http.ResponseWriter,
	r *http.Request,
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
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)

		return
	}

	images, err := storage.DeleteBarber(id)

	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if err != nil {
		serverError(w, "delete barber", err)

		return
	}

	for _, image := range images {
		if err := removeStoredImage(image); err != nil {
			log.Printf("delete barber %d image: %v", id, err)
		}
	}

	if err := session.DeleteForUserContext(r.Context(), session.RoleBarber, id); err != nil {
		serverError(w, "revoke barber sessions", err)
		return
	}

	http.Redirect(w, r, "/admin/barbers", http.StatusSeeOther)
}

// SetBarberStatusHandler suspends or reactivates an account, the
// "suspend or activate barber accounts" item from the README.
//
// Suspending also drops that barber's sessions so an already signed-in
// browser stops working immediately.
func SetBarberStatusHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	idSegment string,
	activate bool,
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
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)

		return
	}

	err := storage.SetBarberStatusContext(r.Context(), id, statusLabel(activate))

	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if err != nil {
		serverError(w, "set barber status", err)

		return
	}

	if !activate {
		if err := session.DeleteForUserContext(r.Context(), session.RoleBarber, id); err != nil {
			serverError(w, "revoke barber sessions", err)
			return
		}
	}

	http.Redirect(w, r, "/admin/barbers", http.StatusSeeOther)
}
