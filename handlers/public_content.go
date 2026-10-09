package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"

	"barber-booking/models"
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

type publicPageMetadata struct {
	Title          string
	Description    string
	Canonical      string
	Type           string
	Image          string
	ImageWidth     int
	ImageHeight    int
	ImageAlt       string
	TwitterCard    string
	FbAppID        string
	StructuredData template.HTML
}

func resolveMetadataURL(origin, pathOrURL string) string {
	if pathOrURL == "" {
		return ""
	}
	if strings.HasPrefix(pathOrURL, "https://") {
		u, err := url.Parse(pathOrURL)
		if err == nil && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(pathOrURL, "\\#") {
			return pathOrURL
		}
	}
	if strings.HasPrefix(pathOrURL, "/") && !strings.HasPrefix(pathOrURL, "//") && !strings.ContainsAny(pathOrURL, "\\#?") {
		if origin != "" {
			u, err := url.Parse(origin)
			if err == nil && u.Scheme == "https" && u.Hostname() != "" {
				u.Path = pathOrURL
				u.RawPath = ""
				u.RawQuery = ""
				u.Fragment = ""
				return u.String()
			}
		}
	}
	return ""
}

// Use configured deployment origins, never request Host or forwarded headers.
// Missing/invalid configuration omits absolute URLs instead of inventing an origin.
func publicMetadata(path, title, description string, extras ...any) publicPageMetadata {
	meta := publicPageMetadata{
		Title:       title,
		Description: description,
		Type:        "website",
		TwitterCard: "summary_large_image",
		FbAppID:     os.Getenv("FB_APP_ID"),
	}
	origin := os.Getenv("SITE_URL")
	if origin == "" {
		origin = os.Getenv("RENDER_EXTERNAL_URL")
	}
	u, err := url.Parse(origin)
	validOrigin := err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Opaque == "" && (u.Path == "" || u.Path == "/") && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(origin, "#")

	if validOrigin && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.ContainsAny(path, "?#\\") {
		u.Path = path
		u.RawPath = ""
		meta.Canonical = u.String()
	}

	for i, extra := range extras {
		switch i {
		case 0:
			if img, ok := extra.(string); ok && img != "" {
				meta.Image = resolveMetadataURL(origin, img)
				if meta.Image != "" {
					meta.ImageWidth = 1200
					meta.ImageHeight = 630
				}
			}
		case 1:
			if alt, ok := extra.(string); ok && alt != "" {
				meta.ImageAlt = alt
			}
		case 2:
			if typ, ok := extra.(string); ok && typ != "" {
				meta.Type = typ
			}
		case 3:
			if card, ok := extra.(string); ok && card != "" {
				meta.TwitterCard = card
			}
		case 4:
			if s, ok := extra.(template.HTML); ok {
				meta.StructuredData = s
			} else if s, ok := extra.(string); ok && s != "" {
				meta.StructuredData = template.HTML(s)
			}
		}
	}
	return meta
}

type webSiteJSONLD struct {
	Context         string         `json:"@context"`
	Type            string         `json:"@type"`
	Name            string         `json:"name"`
	URL             string         `json:"url"`
	PotentialAction map[string]any `json:"potentialAction,omitempty"`
}

func websiteSchema(path string) template.HTML {
	origin := os.Getenv("SITE_URL")
	if origin == "" {
		origin = os.Getenv("RENDER_EXTERNAL_URL")
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return ""
	}
	baseURL := u.Scheme + "://" + u.Host
	data := webSiteJSONLD{
		Context: "https://schema.org",
		Type:    "WebSite",
		Name:    "De Go Salon",
		URL:     baseURL,
		PotentialAction: map[string]any{
			"@type":       "SearchAction",
			"target":      baseURL + "/explore?state={search_term_string}",
			"query-input": "required name=search_term_string",
		},
	}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return template.HTML(string(b))
}

type salonPostalAddress struct {
	Type            string `json:"@type"`
	StreetAddress   string `json:"streetAddress,omitempty"`
	AddressLocality string `json:"addressLocality,omitempty"`
	AddressRegion   string `json:"addressRegion,omitempty"`
	AddressCountry  string `json:"addressCountry"`
}

type salonAggregateRating struct {
	Type        string `json:"@type"`
	RatingValue string `json:"ratingValue"`
	ReviewCount int    `json:"reviewCount"`
}

type hairSalonJSONLD struct {
	Context         string                `json:"@context"`
	Type            string                `json:"@type"`
	Name            string                `json:"name"`
	Image           string                `json:"image,omitempty"`
	Address         *salonPostalAddress   `json:"address,omitempty"`
	PriceRange      string                `json:"priceRange,omitempty"`
	AggregateRating *salonAggregateRating `json:"aggregateRating,omitempty"`
}

func salonSchema(barber models.Barber, rating float64, reviewCount int) template.HTML {
	if barber.ShopName == "" {
		return ""
	}
	origin := os.Getenv("SITE_URL")
	if origin == "" {
		origin = os.Getenv("RENDER_EXTERNAL_URL")
	}
	imageURL := ""
	if barber.ProfileImage != "" {
		imageURL = resolveMetadataURL(origin, barber.ProfileImage)
	} else if barber.PersonalImage != "" {
		imageURL = resolveMetadataURL(origin, barber.PersonalImage)
	}

	data := hairSalonJSONLD{
		Context:    "https://schema.org",
		Type:       "HairSalon",
		Name:       barber.ShopName,
		Image:      imageURL,
		PriceRange: "₦₦",
	}
	if barber.ShopAddress != "" || barber.LocalArea != "" || barber.State != "" {
		data.Address = &salonPostalAddress{
			Type:            "PostalAddress",
			StreetAddress:   barber.ShopAddress,
			AddressLocality: barber.LocalArea,
			AddressRegion:   barber.State,
			AddressCountry:  "NG",
		}
	}
	if reviewCount > 0 && rating > 0 {
		data.AggregateRating = &salonAggregateRating{
			Type:        "AggregateRating",
			RatingValue: fmt.Sprintf("%.1f", rating),
			ReviewCount: reviewCount,
		}
	}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return template.HTML(string(b))
}
