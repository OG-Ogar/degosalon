package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"barber-booking/models"
	"barber-booking/storage"
)

// bookingAlert is the small JSON shape the barber's phone needs to raise the
// "new booking" alert. It deliberately omits customer contact details: the
// alert only has to say that a request arrived and where to review it.
type bookingAlert struct {
	ID       int     `json:"id"`
	Customer string  `json:"customer"`
	Service  string  `json:"service"`
	Date     string  `json:"date"`
	Time     string  `json:"time"`
	Price    float64 `json:"price"`
}

// BarberNotificationsHandler is the poll target behind the live booking alert
// on the barber's dashboard.
//
// The first call (no ?after=) is a baseline: it reports the newest booking id
// and no alerts, so opening the app does not replay every old pending request.
// Later calls pass ?after=<that id> and receive only bookings created since.
func BarberNotificationsHandler(w http.ResponseWriter, r *http.Request, barber models.Barber) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	afterText := strings.TrimSpace(r.URL.Query().Get("after"))
	baseline := afterText == ""
	afterID := 0
	if !baseline {
		value, err := strconv.Atoi(afterText)
		if err != nil || value < 0 {
			journeyError(w, "That alert cursor is not valid.", http.StatusBadRequest)

			return
		}
		afterID = value
	}

	latest, pending, err := storage.PendingBookingsForBarberAfter(r.Context(), barber.ID, afterID)
	if err != nil {
		serverError(w, "load barber booking alerts", err)

		return
	}

	alerts := make([]bookingAlert, 0, len(pending))
	if !baseline {
		for _, booking := range pending {
			alerts = append(alerts, bookingAlert{
				ID:       booking.ID,
				Customer: booking.CustomerName,
				Service:  booking.ServiceName,
				Date:     booking.Date,
				Time:     booking.Time,
				Price:    booking.ServicePrice,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"latest":   latest,
		"bookings": alerts,
	})
}