package storage

import (
	"context"
	"testing"

	"barber-booking/models"
)

func BenchmarkDisplayPreferencesCache(b *testing.B) {
	Reset()
	b.Cleanup(Reset)
	ctx := context.Background()

	prefs := models.DisplayPreferences{
		Theme:        models.ThemeMidnight,
		TextSize:     models.TextSizeLarge,
		NotifyAlerts: true,
	}
	_, err := SaveDisplayPreferencesContext(ctx, "customer", 42, prefs)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DisplayPreferencesContext(ctx, "customer", 42)
	}
}

