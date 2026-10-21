package models

// DisplayPreferences are a signed-in user's persisted display choices: the
// colour theme and the accessibility text size, plus the alert opt-in. The
// values are always normalised through the allowlists below before they are
// stored or rendered, so an unrecognised value can never reach the markup.
type DisplayPreferences struct {
	Theme        string
	TextSize     string
	NotifyAlerts bool
}

// The allowlisted theme identifiers. They match the html[data-theme="…"]
// selectors in frontreadme.md §2.2.
const (
	ThemeForest   = "forest"
	ThemeNavy     = "navy"
	ThemeMidnight = "midnight"
	ThemeDaylight = "daylight"
	ThemeOcean    = "ocean"
	ThemeStone    = "stone"
)

// The allowlisted text-size identifiers. They match the html[data-text-size="…"]
// selectors in frontreadme.md §2.3C.
const (
	TextSizeStandard = "standard"
	TextSizeLarge    = "large"
	TextSizeXLarge   = "xlarge"
	TextSizeXXLarge  = "xxlarge"
)

// Themes and TextSizes are the server's allowlists. The order is also the order
// the picker presents them in.
var Themes = []string{ThemeForest, ThemeNavy, ThemeMidnight, ThemeDaylight, ThemeOcean, ThemeStone}
var TextSizes = []string{TextSizeStandard, TextSizeLarge, TextSizeXLarge, TextSizeXXLarge}

// DefaultDisplayPreferences is the fallback for a visitor with no stored
// choice: the Forest palette at the standard reading size.
func DefaultDisplayPreferences() DisplayPreferences {
	return DisplayPreferences{Theme: ThemeForest, TextSize: TextSizeStandard}
}

// ValidTheme reports whether theme is on the allowlist.
func ValidTheme(theme string) bool { return containsString(Themes, theme) }

// ValidTextSize reports whether size is on the allowlist.
func ValidTextSize(size string) bool { return containsString(TextSizes, size) }

// NormalizeDisplayPreferences replaces any unrecognised theme or text size with
// the default. It never returns an empty theme or size, so callers can render
// the result directly (frontreadme.md §2.3A).
func NormalizeDisplayPreferences(preferences DisplayPreferences) DisplayPreferences {
	if !ValidTheme(preferences.Theme) {
		preferences.Theme = ThemeForest
	}
	if !ValidTextSize(preferences.TextSize) {
		preferences.TextSize = TextSizeStandard
	}
	return preferences
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
