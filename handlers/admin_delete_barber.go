package handlers

import (
	"errors"
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
		journeyError(w, "Invalid barber ID", http.StatusBadRequest)

		return
	}

	barber, err := storage.FindBarberByIDContext(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load admin barber", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Page not found.", http.StatusNotFound)

		return
	}

	barber.PasswordHash = ""

	render(w, "admin/confirm-delete.html", barberView{
		Barber: barber,
		CSRF:   s.CSRFToken,
	})
}

// DeleteBarberHandler archives an account while retaining customer history.
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
		journeyError(w, "Invalid barber ID", http.StatusBadRequest)

		return
	}

	err := storage.ArchiveBarber(r.Context(), id)
	if errors.Is(err, storage.ErrOutstandingWork) {
		barber, loadErr := storage.FindBarberByIDContext(r.Context(), id)
		if loadErr != nil {
			serverError(w, "load archive conflict", loadErr)
			return
		}
		barber.PasswordHash = ""
		renderStatus(w, http.StatusConflict, "admin/confirm-delete.html", barberView{Barber: barber, CSRF: s.CSRFToken, Error: err.Error()})
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, "archive barber", err)
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
		journeyError(w, "Invalid barber ID", http.StatusBadRequest)

		return
	}

	err := storage.SetBarberStatusContext(r.Context(), id, statusLabel(activate))

	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Page not found.", http.StatusNotFound)

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
