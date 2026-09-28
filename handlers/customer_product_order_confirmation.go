package handlers

import (
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

	order, found := storage.FindProductOrderByID(orderID)

	if !found {
		http.NotFound(w, r)
		return
	}

	// A customer can only view their own order.
	if order.CustomerID != customer.ID {
		http.NotFound(w, r)
		return
	}

	orderItems := storage.ListProductOrderItems(order.ID)

	items := make([]customerProductOrderConfirmationItem, 0, len(orderItems))

	for _, item := range orderItems {
		product, found := storage.FindProductByID(item.ProductID)

		if !found {
			continue
		}

		barber, found := storage.FindBarberByID(item.BarberID)

		if !found {
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
