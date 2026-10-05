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
	if !requireCSRF(w, r, s) {
		return
	}

	order, err := storage.CheckoutCartWithKey(r.Context(), customer, r.PostFormValue("checkout_key"))
	if err != nil {
		var checkoutErr storage.CheckoutError
		switch {
		case errors.Is(err, storage.ErrInvalidCheckoutKey):
			renderCustomerCart(w, r, customer, s, http.StatusBadRequest, "This checkout form is incomplete or expired. Review your cart and submit the new form below.")
		case errors.Is(err, storage.ErrNotFound):
			http.Redirect(w, r, "/customer/cart", http.StatusSeeOther)
		case errors.As(err, &checkoutErr):
			renderCustomerCart(w, r, customer, s, http.StatusConflict, checkoutErr.Error())
		default:
			serverError(w, "checkout customer cart", err)
		}
		return
	}
	http.Redirect(w, r, "/customer/order-confirmation/"+strconv.Itoa(order.ID), http.StatusSeeOther)
}
