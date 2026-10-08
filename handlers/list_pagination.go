package handlers

import (
	"barber-booking/storage"
	"net/http"
	"strconv"
	"strings"
)

type listPagination struct {
	PreviousLink, NextLink, FirstLink    string
	PreviousLabel, NextLabel, FirstLabel string
}

func readPageCursor(w http.ResponseWriter, r *http.Request) (storage.PageCursor, bool) {
	cursor := storage.PageCursor{}
	for key, target := range map[string]*int{"after": &cursor.After, "before": &cursor.Before} {
		values, found := r.URL.Query()[key]
		if !found {
			continue
		}
		if len(values) != 1 {
			journeyError(w, "This results page is invalid. Return to the first page.", 400)
			return cursor, false
		}
		value, ok := parseID(values[0])
		if !ok {
			journeyError(w, "This results page is invalid. Return to the first page.", 400)
			return cursor, false
		}
		*target = value
	}
	cursor, err := cursor.Normalize()
	if err != nil {
		journeyError(w, "Choose one results page direction.", 400)
		return cursor, false
	}
	return cursor, true
}
func paginationLinks(r *http.Request, cursor storage.PageCursor, first, last int, ascending, previous, next bool) listPagination {
	link := func(after, before int) string {
		query := r.URL.Query()
		query.Del("after")
		query.Del("before")
		if after > 0 {
			query.Set("after", strconv.Itoa(after))
		}
		if before > 0 {
			query.Set("before", strconv.Itoa(before))
		}
		return discoveryPath(r.URL.Path, query)
	}
	result := listPagination{FirstLink: link(0, 0)}
	if previous {
		if ascending {
			result.PreviousLink = link(0, first)
		} else {
			result.PreviousLink = link(first, 0)
		}
	}
	if next {
		if ascending {
			result.NextLink = link(last, 0)
		} else {
			result.NextLink = link(0, last)
		}
	}
	return result
}

func readListSearch(w http.ResponseWriter, r *http.Request) (string, bool) {
	values, exists := r.URL.Query()["q"]
	if exists && len(values) != 1 {
		journeyError(w, "Enter one search term.", 400)
		return "", false
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 200 {
		journeyError(w, "Use a search term of at most 200 bytes.", 400)
		return "", false
	}
	return query, true
}
