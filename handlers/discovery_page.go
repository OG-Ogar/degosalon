package handlers

import (
	"barber-booking/internal/geography"
	"barber-booking/storage"
	"net/http"
	"net/url"
	"strconv"
)

type discoveryPagination struct {
	PreviousLink, NextLink, FirstLink string
	After, Before                     int
}

func discoveryCardPath(base string, id int, state, area string, after, before int) string {
	query := url.Values{"state": {state}}
	if area != "" {
		query.Set("area", area)
	}
	if after > 0 {
		query.Set("after", strconv.Itoa(after))
	}
	if before > 0 {
		query.Set("before", strconv.Itoa(before))
	}
	if id > 0 {
		base += "/" + strconv.Itoa(id)
	}
	return discoveryPath(base, query)
}
func readDiscoveryPage(w http.ResponseWriter, r *http.Request, base string) (storage.DiscoveryPage, string, string, discoveryPagination, bool) {
	state, area, ok := discoveryLocation(r)
	if !ok {
		journeyError(w, "Choose a valid state/FCT and an LGA or area council within it.", http.StatusBadRequest)
		return storage.DiscoveryPage{}, "", "", discoveryPagination{}, false
	}
	filter := storage.DiscoveryFilter{StateID: geography.StateID(state), AreaID: geography.AreaID(state, area), Limit: storage.DiscoveryPageSize}
	for key, target := range map[string]*int{"after": &filter.After, "before": &filter.Before} {
		values, exists := r.URL.Query()[key]
		if !exists {
			continue
		}
		if len(values) != 1 {
			journeyError(w, "This results page is invalid. Choose your location again.", http.StatusBadRequest)
			return storage.DiscoveryPage{}, "", "", discoveryPagination{}, false
		}
		value, valid := parseID(values[0])
		if !valid {
			journeyError(w, "This results page is invalid. Choose your location again.", http.StatusBadRequest)
			return storage.DiscoveryPage{}, "", "", discoveryPagination{}, false
		}
		*target = value
	}
	if filter.After > 0 && filter.Before > 0 {
		journeyError(w, "Choose one results page direction.", http.StatusBadRequest)
		return storage.DiscoveryPage{}, "", "", discoveryPagination{}, false
	}
	page, err := storage.DiscoveryPageResult(r.Context(), filter)
	if err != nil {
		serverError(w, "load scoped discovery", err)
		return storage.DiscoveryPage{}, "", "", discoveryPagination{}, false
	}
	pagination := discoveryPagination{FirstLink: discoveryCardPath(base, 0, state, area, 0, 0), After: filter.After, Before: filter.Before}
	if len(page.Items) == 0 && page.Total > 0 && (filter.After > 0 || filter.Before > 0) {
		http.Redirect(w, r, pagination.FirstLink, http.StatusSeeOther)
		return storage.DiscoveryPage{}, "", "", pagination, false
	}
	if page.HasPrevious {
		pagination.PreviousLink = discoveryCardPath(base, 0, state, area, 0, page.Items[0].ID)
	}
	if page.HasNext {
		pagination.NextLink = discoveryCardPath(base, 0, state, area, page.Items[len(page.Items)-1].ID, 0)
	}
	return page, state, area, pagination, true
}
