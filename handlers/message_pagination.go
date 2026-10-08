package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"net/http"
)

func renderMessagePage(w http.ResponseWriter, r *http.Request, barberID, customerID int) ([]models.Message, listPagination, bool) {
	cursor, ok := readPageCursor(w, r)
	if !ok {
		return nil, listPagination{}, false
	}
	cursor.Limit = 100
	page, err := storage.MessagePage(r.Context(), barberID, customerID, cursor)
	if err != nil {
		serverError(w, "load conversation messages", err)
		return nil, listPagination{}, false
	}
	links := paginationLinks(r, cursor, 0, 0, false, false, false)
	if len(page.Items) > 0 {
		links = paginationLinks(r, cursor, page.Items[0].ID, page.Items[len(page.Items)-1].ID, false, page.HasPrevious, page.HasNext)
	} else if cursor.Before > 0 || cursor.After > 0 {
		http.Redirect(w, r, links.FirstLink, http.StatusSeeOther)
		return nil, links, false
	}
	links.PreviousLabel = "Newer messages"
	links.NextLabel = "Older messages"
	links.FirstLabel = "Latest messages"
	for i, j := 0, len(page.Items)-1; i < j; i, j = i+1, j-1 {
		page.Items[i], page.Items[j] = page.Items[j], page.Items[i]
	}
	return page.Items, links, true
}
