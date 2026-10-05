package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"barber-booking/models"
	"barber-booking/storage"
)

func CreateBarberReviewHandler(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData, idSegment string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !requireCSRF(w, r, s) {
		return
	}
	barberID, ok := parseID(idSegment)
	if !ok {
		http.NotFound(w, r)
		return
	}
	barber, lookupErr := storage.FindBarberByIDContext(r.Context(), barberID)
	if lookupErr != nil && !errors.Is(lookupErr, storage.ErrNotFound) {
		serverError(w, "load review barber", lookupErr)
		return
	}
	if errors.Is(lookupErr, storage.ErrNotFound) || !barber.IsActive() {
		http.NotFound(w, r)
		return
	}
	rating, err := strconv.Atoi(r.PostFormValue("rating"))
	if err != nil || rating < 1 || rating > 5 {
		http.Error(w, "Choose a rating from 1 to 5.", http.StatusBadRequest)
		return
	}
	_, err = storage.CreateReview(r.Context(), models.Review{BarberID: barberID, CustomerID: customer.ID, CustomerName: customer.FullName, Rating: rating, Comment: truncateText(r.PostFormValue("comment"))})
	if errors.Is(err, storage.ErrForbidden) {
		http.Error(w, "Reviews are available after a completed appointment, one review per booking.", http.StatusForbidden)
		return
	}
	if err != nil {
		serverError(w, "create barber review", err)
		return
	}
	http.Redirect(w, r, "/customer/barbers/"+idSegment+"#reviews", http.StatusSeeOther)
}
