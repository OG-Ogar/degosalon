package handlers

import (
	"barber-booking/internal/geography"
	"net/http"
	"strings"
)

// Accept existing name-based links as well as stable IDs from the new controls.
func discoveryLocation(r *http.Request) (string, string, bool) {
	state, area := strings.TrimSpace(r.URL.Query().Get("state")), strings.TrimSpace(r.URL.Query().Get("area"))
	if state == "" {
		return "", "", area == ""
	}
	s, ok := geography.FindState(state)
	if !ok {
		return state, area, false
	}
	if area == "" {
		return s.Name, "", true
	}
	a, ok := geography.FindArea(s, area)
	if !ok {
		return s.Name, area, false
	}
	return s.Name, a.Name, true
}
