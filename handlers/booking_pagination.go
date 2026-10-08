package handlers

import (
	"barber-booking/storage"
	"net/http"
)

func readBookingGroup(w http.ResponseWriter, r *http.Request) (string, bool) {
	values, exists := r.URL.Query()["group"]
	if exists && len(values) != 1 {
		journeyError(w, "Choose one appointment group.", 400)
		return "", false
	}
	group := r.URL.Query().Get("group")
	switch group {
	case "", "all", "pending", "upcoming", "followup", "finished":
		return group, true
	}
	journeyError(w, "Choose a valid appointment group.", 400)
	return "", false
}
func bookingPagination(r *http.Request, cursor storage.PageCursor, page storage.Page[storage.BookingCard]) listPagination {
	first, last := 0, 0
	if len(page.Items) > 0 {
		first, last = page.Items[0].ID, page.Items[len(page.Items)-1].ID
	}
	return paginationLinks(r, cursor, first, last, false, page.HasPrevious, page.HasNext)
}
