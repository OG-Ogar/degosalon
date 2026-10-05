package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"errors"
	"net/http"
)

func BarberOrderItemActionHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData, idSegment, action string) {
	target, ok := map[string]string{"ready": models.FulfillmentReady, "collect": models.FulfillmentCollected, "cancel": models.FulfillmentCancelled}[action]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if !requireCSRF(w, r, s) {
		return
	}
	id, ok := parseID(idSegment)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := storage.FulfillOrderItem(r.Context(), id, barber.ID, target)
	switch {
	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrForbidden):
		http.NotFound(w, r)
		return
	case errors.Is(err, storage.ErrInvalidTransition):
		http.Error(w, "This item changed or is already closed. Refresh the dashboard before trying again.", http.StatusConflict)
		return
	case err != nil:
		serverError(w, "fulfill seller order item", err)
		return
	}
	http.Redirect(w, r, "/barber/dashboard", http.StatusSeeOther)
}
