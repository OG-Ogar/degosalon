package storage

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"barber-booking/models"
)

// displayPreferences holds the signed-in user's choice in the in-memory store
// used by tests and local development. It mirrors the user_preferences table.
var displayPreferences = map[string]models.DisplayPreferences{}

// preferenceKey separates the workspaces: administrators share user id 0, so
// the role is what keeps an admin, a barber and a customer from sharing a row.
func preferenceKey(role string, userID int) string {
	return role + ":" + strconv.Itoa(userID)
}

// DisplayPreferencesContext returns the stored display preferences for a
// signed-in user, or the defaults when none has been saved. The result is
// always normalised, so a legacy row holding an unknown value falls back
// safely instead of reaching the markup (frontreadme.md §2.3A).
func DisplayPreferencesContext(ctx context.Context, role string, userID int) (models.DisplayPreferences, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return models.DisplayPreferences{}, err
	}

	if databaseEnabled() {
		var theme, size string
		var alerts bool
		err := db.QueryRowContext(ctx,
			"select theme,text_size,notify_alerts from degosalon_internal.user_preferences where user_role=$1 and user_id=$2",
			role, userID).Scan(&theme, &size, &alerts)
		if errors.Is(err, sql.ErrNoRows) {
			return models.DefaultDisplayPreferences(), nil
		}
		if err != nil {
			return models.DisplayPreferences{}, err
		}
		return models.NormalizeDisplayPreferences(models.DisplayPreferences{
			Theme: theme, TextSize: size, NotifyAlerts: alerts,
		}), nil
	}

	mu.RLock()
	defer mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return models.DisplayPreferences{}, err
	}
	// A missing key is the zero value, which normalises to the defaults.
	return models.NormalizeDisplayPreferences(displayPreferences[preferenceKey(role, userID)]), nil
}

// SaveDisplayPreferencesContext stores a user's choice after normalising it,
// so only allowlisted values can reach the database.
func SaveDisplayPreferencesContext(ctx context.Context, role string, userID int, preferences models.DisplayPreferences) (models.DisplayPreferences, error) {
	preferences = models.NormalizeDisplayPreferences(preferences)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return models.DisplayPreferences{}, err
	}

	if databaseEnabled() {
		_, err := db.ExecContext(ctx,
			`insert into degosalon_internal.user_preferences(user_role,user_id,theme,text_size,notify_alerts,updated_at)
			 values($1,$2,$3,$4,$5,now())
			 on conflict(user_role,user_id) do update set
			   theme=excluded.theme, text_size=excluded.text_size,
			   notify_alerts=excluded.notify_alerts, updated_at=now()`,
			role, userID, preferences.Theme, preferences.TextSize, preferences.NotifyAlerts)
		if err != nil {
			return models.DisplayPreferences{}, err
		}
		return preferences, nil
	}

	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return models.DisplayPreferences{}, err
	}
	displayPreferences[preferenceKey(role, userID)] = preferences
	return preferences, nil
}
