package storage

import (
	"context"
	"testing"

	"barber-booking/models"
)

func TestDisplayPreferencesRoundTrip(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ctx := context.Background()

	// A user with no stored row reads back the defaults.
	got, err := DisplayPreferencesContext(ctx, "customer", 7)
	if err != nil {
		t.Fatal(err)
	}
	if got != models.DefaultDisplayPreferences() {
		t.Fatalf("default read = %+v", got)
	}

	// A saved choice is returned exactly.
	want := models.DisplayPreferences{Theme: models.ThemeOcean, TextSize: models.TextSizeXLarge, NotifyAlerts: true}
	saved, err := SaveDisplayPreferencesContext(ctx, "customer", 7, want)
	if err != nil {
		t.Fatal(err)
	}
	if saved != want {
		t.Fatalf("saved = %+v", saved)
	}
	got, err = DisplayPreferencesContext(ctx, "customer", 7)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip = %+v", got)
	}

	// A crafted value is normalised before it is stored.
	if _, err := SaveDisplayPreferencesContext(ctx, "customer", 7, models.DisplayPreferences{Theme: "<script>", TextSize: "huge"}); err != nil {
		t.Fatal(err)
	}
	got, err = DisplayPreferencesContext(ctx, "customer", 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != models.ThemeForest || got.TextSize != models.TextSizeStandard {
		t.Fatalf("normalised write = %+v", got)
	}

	// The role keeps an administrator (user id 0) separate from a customer row.
	admin, err := DisplayPreferencesContext(ctx, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	if admin != models.DefaultDisplayPreferences() {
		t.Fatalf("preferences leaked across roles: %+v", admin)
	}
}
