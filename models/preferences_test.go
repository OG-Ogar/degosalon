package models

import "testing"

func TestNormalizeDisplayPreferences(t *testing.T) {
	cases := []struct {
		name string
		in   DisplayPreferences
		want DisplayPreferences
	}{
		{
			name: "zero value falls back to the defaults",
			in:   DisplayPreferences{},
			want: DisplayPreferences{Theme: ThemeForest, TextSize: TextSizeStandard},
		},
		{
			name: "allowlisted values pass through unchanged",
			in:   DisplayPreferences{Theme: ThemeOcean, TextSize: TextSizeXXLarge, NotifyAlerts: true},
			want: DisplayPreferences{Theme: ThemeOcean, TextSize: TextSizeXXLarge, NotifyAlerts: true},
		},
		{
			name: "a crafted theme is rejected but the size is kept",
			in:   DisplayPreferences{Theme: `midnight" onload="x`, TextSize: TextSizeLarge},
			want: DisplayPreferences{Theme: ThemeForest, TextSize: TextSizeLarge},
		},
		{
			name: "an unknown text size falls back, the theme is kept",
			in:   DisplayPreferences{Theme: ThemeStone, TextSize: "gigantic"},
			want: DisplayPreferences{Theme: ThemeStone, TextSize: TextSizeStandard},
		},
	}

	for _, tc := range cases {
		if got := NormalizeDisplayPreferences(tc.in); got != tc.want {
			t.Fatalf("%s: got %+v want %+v", tc.name, got, tc.want)
		}
	}
}

func TestValidThemeAndTextSize(t *testing.T) {
	for _, theme := range Themes {
		if !ValidTheme(theme) {
			t.Fatalf("allowlisted theme %q rejected", theme)
		}
	}
	for _, invalid := range []string{"", "Forest", "dark", "forest;color:red"} {
		if ValidTheme(invalid) {
			t.Fatalf("theme %q accepted", invalid)
		}
	}
	for _, size := range TextSizes {
		if !ValidTextSize(size) {
			t.Fatalf("allowlisted text size %q rejected", size)
		}
	}
	for _, invalid := range []string{"", "Standard", "xxl", "standard large"} {
		if ValidTextSize(invalid) {
			t.Fatalf("text size %q accepted", invalid)
		}
	}
}
