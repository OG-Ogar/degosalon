package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

type customerProductOrderConfirmationView struct {
	Order models.ProductOrder
	Items []models.ProductOrderItem
	CSRF  string
}

func CustomerProductOrderConfirmationHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	orderID, ok := parseID(idSegment)

	if !ok {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}

	order, err := storage.FindProductOrderByIDResult(r.Context(), orderID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load customer order", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}

	// A customer can only view their own order.
	if order.CustomerID != customer.ID {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}

	orderItems, err := storage.ListProductOrderItems(r.Context(), order.ID)
	if err != nil {
		serverError(w, "load order items", err)
		return
	}

	view := customerProductOrderConfirmationView{
		Order: order,
		Items: orderItems,
		CSRF:  s.CSRFToken,
	}

	render(
		w,
		"customer/order-confirmation.html",
		view,
	)
}
