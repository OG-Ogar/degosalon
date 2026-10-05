package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"barber-booking/models"
	"barber-booking/storage"
)

func CustomerProductOrderHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !requireCSRF(w, r, s) { return }

	order, err := storage.CheckoutCart(customer)
	if err != nil {
		var checkoutErr storage.CheckoutError
		switch {
		case errors.Is(err, storage.ErrNotFound):
			http.Redirect(w, r, "/customer/cart", http.StatusSeeOther)
		case errors.As(err, &checkoutErr):
			http.Error(w, checkoutErr.Error(), http.StatusConflict)
		default:
			serverError(w, "checkout customer cart", err)
		}
		return
	}
	http.Redirect(w, r, "/customer/order-confirmation/"+strconv.Itoa(order.ID), http.StatusSeeOther)
}
