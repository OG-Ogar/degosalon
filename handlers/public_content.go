package handlers

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

type publicContentView struct {
	Title, Description, Kind, Path string
	Placeholder                    bool
}

var publicContentPages = map[string]publicContentView{
	"/help":    {Title: "Help", Description: "Help with finding a barber, booking, messages and product collection.", Kind: "help", Path: "/help", Placeholder: true},
	"/support": {Title: "Support", Description: "Contact De Go Salon support by telephone.", Kind: "support", Path: "/support"},
	"/privacy": {Title: "Privacy", Description: "Privacy information for De Go Salon.", Kind: "privacy", Path: "/privacy", Placeholder: true},
	"/terms":   {Title: "Terms", Description: "Terms information for De Go Salon.", Kind: "terms", Path: "/terms", Placeholder: true},
}

func PublicContentHandler(w http.ResponseWriter, r *http.Request) {
	page, ok := publicContentPages[r.URL.Path]
	if !ok {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	// Legal pages remain explicit placeholders by owner request. Approved
	// support contacts can be indexed independently of those policies.
	if page.Placeholder {
		w.Header().Set("X-Robots-Tag", "noindex, follow")
	}
	render(w, "public-content.html", page)
}

type publicPageMetadata struct{ Title, Description, Canonical string }

// Use configured deployment origins, never request Host or forwarded headers.
// Missing/invalid configuration omits absolute URLs instead of inventing an origin.
func publicMetadata(path, title, description string) publicPageMetadata {
	meta := publicPageMetadata{Title: title, Description: description}
	origin := os.Getenv("SITE_URL")
	if origin == "" {
		origin = os.Getenv("RENDER_EXTERNAL_URL")
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(origin, "#") {
		return meta
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "?#\\") {
		return meta
	}
	u.Path = path
	u.RawPath = ""
	meta.Canonical = u.String()
	return meta
}
