package handlers

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

func discoveryQuery(r *http.Request) url.Values {
	query := url.Values{}
	for _, key := range []string{"state", "area", "after", "before"} {
		value := strings.TrimSpace(r.URL.Query().Get(key))
		if value == "" && r.Method == http.MethodPost {
			value = strings.TrimSpace(r.PostFormValue(key))
		}
		if key == "after" || key == "before" {
			if _, ok := parseID(value); !ok {
				continue
			}
		}
		if value != "" {
			query.Set(key, value)
		}
	}
	return query
}

func discoveryPath(base string, query url.Values) string {
	address := url.URL{Path: base, RawQuery: query.Encode()}
	return address.String()
}

func customerBookingPath(id, style int, query url.Values) string {
	copy := url.Values{}
	for key, values := range query {
		copy[key] = append([]string(nil), values...)
	}
	if style > 0 {
		copy.Set("hairstyle", strconv.Itoa(style))
	}
	address := url.URL{Path: "/customer/barbers/" + strconv.Itoa(id), RawQuery: copy.Encode(), Fragment: "booking"}
	return address.String()
}

func customerLoginPath(next string) string {
	next = safeCustomerNext(next)
	if next == "" {
		return "/customer/login"
	}
	return discoveryPath("/customer/login", url.Values{"next": {next}})
}

// Only GET destinations within the customer workspace are eligible returns.
// Reject external, encoded/unclean paths and authentication loops.
func safeCustomerNext(next string) string {
	address, err := url.Parse(strings.TrimSpace(next))
	if err != nil || address.IsAbs() || address.Host != "" || address.User != nil || strings.ContainsAny(address.Path, "\\\r\n") || path.Clean(address.Path) != address.Path || strings.Contains(address.EscapedPath(), "%") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(address.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "customer" {
		return ""
	}
	allowed := false
	switch parts[1] {
	case "dashboard", "products", "cart", "bookings":
		allowed = len(parts) == 2
	case "profile":
		allowed = len(parts) == 2 || (len(parts) == 3 && parts[2] == "edit")
	case "barbers", "chats":
		allowed = len(parts) == 2
		if len(parts) == 3 {
			_, allowed = parseID(parts[2])
		}
	}
	if !allowed {
		return ""
	}
	query := address.Query()
	clean := url.Values{}
	for _, key := range []string{"state", "area", "hairstyle", "barber", "product", "page", "q", "after", "before"} {
		if value := query.Get(key); value != "" {
			if key == "after" || key == "before" {
				if _, ok := parseID(value); !ok {
					continue
				}
			}
			clean.Set(key, value)
		}
	}
	address.RawQuery = clean.Encode()
	if address.Fragment != "booking" && address.Fragment != "reviews" && address.Fragment != "products" {
		address.Fragment = ""
	}
	return address.String()
}
