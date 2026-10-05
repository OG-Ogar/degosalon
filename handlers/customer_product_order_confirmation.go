package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

type customerProductOrderConfirmationItem struct {
	Item    models.ProductOrderItem
	Product models.Product
	Barber  models.Barber
}

type customerProductOrderConfirmationView struct {
	Order models.ProductOrder
	Items []customerProductOrderConfirmationItem
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
		http.NotFound(w, r)
		return
	}

	order, err := storage.FindProductOrderByIDResult(r.Context(), orderID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load customer order", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}

	// A customer can only view their own order.
	if order.CustomerID != customer.ID {
		http.NotFound(w, r)
		return
	}

	orderItems, err := storage.ListProductOrderItems(r.Context(), order.ID)
	if err != nil {
		serverError(w, "load order items", err)
		return
	}

	items := make([]customerProductOrderConfirmationItem, 0, len(orderItems))

	for _, item := range orderItems {
		product, err := storage.FindProductByIDResult(r.Context(), item.ProductID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "load order product", err)
			return
		}
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}

		barber, err := storage.FindBarberByIDContext(r.Context(), item.BarberID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "load order seller", err)
			return
		}
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}

		items = append(items, customerProductOrderConfirmationItem{
			Item:    item,
			Product: product,
			Barber:  barber,
		})
	}

	view := customerProductOrderConfirmationView{
		Order: order,
		Items: items,
		CSRF:  s.CSRFToken,
	}

	render(
		w,
		"customer/order-confirmation.html",
		view,
	)
}
