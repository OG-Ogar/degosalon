package handlers

import (
	"barber-booking/storage"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type listPagination struct {
	Name                                 string
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

// Independent cursors keep profile sections separate from discovery results.
func readNamedPageCursor(w http.ResponseWriter, r *http.Request, prefix string) (storage.PageCursor, bool) {
	copy := r.Clone(r.Context())
	address := *r.URL
	query := address.Query()
	query.Del("after")
	query.Del("before")
	for _, key := range []string{"after", "before"} {
		if values, ok := r.URL.Query()[prefix+"_"+key]; ok && !(len(values) == 1 && values[0] == "") {
			query[key] = values
		}
	}
	address.RawQuery = query.Encode()
	copy.URL = &address
	return readPageCursor(w, copy)
}
func namedPaginationLinks(r *http.Request, prefix, fragment string, cursor storage.PageCursor, first, last int, ascending, previous, next bool) listPagination {
	copy := r.Clone(r.Context())
	address := *r.URL
	query := address.Query()
	query.Del("after")
	query.Del("before")
	query.Del(prefix + "_after")
	query.Del(prefix + "_before")
	address.RawQuery = query.Encode()
	copy.URL = &address
	links := paginationLinks(copy, cursor, first, last, ascending, previous, next)
	convert := func(link string) string {
		if link == "" {
			return ""
		}
		address, _ := url.Parse(link)
		query := address.Query()
		for _, key := range []string{"after", "before"} {
			if value := query.Get(key); value != "" {
				query.Set(prefix+"_"+key, value)
			}
			query.Del(key)
			if values, ok := r.URL.Query()[key]; ok {
				query[key] = values
			}
		}
		address.RawQuery = query.Encode()
		address.Fragment = fragment
		return address.String()
	}
	links.PreviousLink = convert(links.PreviousLink)
	links.NextLink = convert(links.NextLink)
	links.FirstLink = convert(links.FirstLink)
	links.PreviousLabel = "Previous " + prefix
	links.NextLabel = "Next " + prefix
	links.FirstLabel = "First " + prefix
	links.Name = prefix + " pages"
	return links
}
