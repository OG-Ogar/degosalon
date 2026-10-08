package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"net/http"
)

func orderPagination(r *http.Request, prefix string, cursor storage.PageCursor, items []models.ProductOrder, previous, next bool) listPagination {
	first, last := 0, 0
	if len(items) > 0 {
		first, last = items[0].ID, items[len(items)-1].ID
	}
	return namedPaginationLinks(r, prefix, "orders", cursor, first, last, false, previous, next)
}
func orderItemPagination(r *http.Request, cursor storage.PageCursor, items []models.ProductOrderItem, previous, next bool) listPagination {
	first, last := 0, 0
	if len(items) > 0 {
		first, last = items[0].ID, items[len(items)-1].ID
	}
	return namedPaginationLinks(r, "orders", "orders", cursor, first, last, false, previous, next)
}
func readFulfillmentFilter(w http.ResponseWriter, r *http.Request) (string, bool) {
	values, exists := r.URL.Query()["fulfillment"]
	if exists && len(values) != 1 {
		journeyError(w, "Choose one fulfillment state.", 400)
		return "", false
	}
	status := r.URL.Query().Get("fulfillment")
	switch status {
	case "", "all", "Pending", "Ready", "Collected", "Cancelled":
		return status, true
	}
	journeyError(w, "Choose a valid fulfillment state.", 400)
	return "", false
}
