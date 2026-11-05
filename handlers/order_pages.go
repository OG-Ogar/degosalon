package handlers

import (
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

type customerOrdersView struct {
	CSRF          string
	Pagination    listPagination
	ProductOrders []models.ProductOrder
}

func CustomerOrdersHandler(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	cursor, ok := readNamedPageCursor(w, r, "orders")
	if !ok {
		return
	}
	page, err := storage.CustomerOrderPage(r.Context(), customer.ID, cursor)
	if err != nil {
		serverError(w, "load customer order page", err)
		return
	}
	links := orderPagination(r, "orders", cursor, page.Items, page.HasPrevious, page.HasNext)
	if len(page.Items) == 0 && (cursor.Before > 0 || cursor.After > 0) {
		http.Redirect(w, r, links.FirstLink, http.StatusSeeOther)
		return
	}
	render(w, "customer/orders.html", customerOrdersView{CSRF: s.CSRFToken, Pagination: links, ProductOrders: page.Items})
}

type barberOrdersView struct {
	models.Barber
	CSRF          string
	Pagination    listPagination
	OrderStatus   string
	ProductOrders []models.ProductOrderItem
}

func BarberOrdersHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	cursor, ok := readNamedPageCursor(w, r, "orders")
	if !ok {
		return
	}
	status, ok := readFulfillmentFilter(w, r)
	if !ok {
		return
	}
	page, err := storage.SellerOrderItemPage(r.Context(), barber.ID, status, cursor)
	if err != nil {
		serverError(w, "load seller order page", err)
		return
	}
	links := orderItemPagination(r, cursor, page.Items, page.HasPrevious, page.HasNext)
	if len(page.Items) == 0 && (cursor.Before > 0 || cursor.After > 0) {
		http.Redirect(w, r, links.FirstLink, http.StatusSeeOther)
		return
	}
	barber.PasswordHash = ""
	render(w, "barber/orders.html", barberOrdersView{Barber: barber, CSRF: s.CSRFToken, Pagination: links, OrderStatus: status, ProductOrders: page.Items})
}
