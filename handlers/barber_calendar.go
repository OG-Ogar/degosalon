package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"barber-booking/models"
	"barber-booking/storage"
)

type calendarDayView struct {
	models.DayHours
	Index int
	Name  string
}
type calendarView struct {
	Calendar             models.Calendar
	Days                 []calendarDayView
	CSRF, Error, TimeOff string
}

func BarberCalendarHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	calendar, err := storage.CalendarForBarber(r.Context(), barber.ID)
	if err != nil {
		serverError(w, "load barber calendar", err)
		return
	}
	status := http.StatusOK
	problem := ""
	if r.Method == http.MethodPost {
		if !requireCSRF(w, r, s) {
			return
		}
		calendar.Timezone = strings.TrimSpace(r.PostFormValue("timezone"))
		var e1, e2 error
		calendar.LeadMinutes, e1 = strconv.Atoi(r.PostFormValue("lead"))
		calendar.HorizonDays, e2 = strconv.Atoi(r.PostFormValue("horizon"))
		if e1 != nil || e2 != nil {
			problem = "Choose whole numbers for lead time and advance booking."
		}
		for i := range calendar.Week {
			prefix := "day_" + strconv.Itoa(i) + "_"
			calendar.Week[i] = models.DayHours{Closed: r.PostFormValue(prefix+"closed") == "yes", Open: r.PostFormValue(prefix + "open"), Close: r.PostFormValue(prefix + "close"), BreakStart: r.PostFormValue(prefix + "break_start"), BreakEnd: r.PostFormValue(prefix + "break_end")}
		}
		calendar.TimeOff = nil
		for _, line := range strings.Split(strings.TrimSpace(r.PostFormValue("time_off")), "\n") {
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}
			if len(parts) != 3 {
				problem = "Enter time off as date, start, and end, one period per line."
				break
			}
			calendar.TimeOff = append(calendar.TimeOff, models.TimeOff{Date: parts[0], Start: parts[1], End: parts[2]})
		}
		if problem == "" {
			if err := storage.ValidateCalendar(calendar); err != nil {
				problem = err.Error()
			}
		}
		if problem == "" {
			err = storage.SaveBarberCalendar(r.Context(), barber.ID, calendar)
			if errors.Is(err, storage.ErrScheduleConflict) {
				problem = "This change overlaps an existing appointment. Resolve that appointment first."
				status = http.StatusConflict
			} else if err != nil {
				serverError(w, "save barber calendar", err)
				return
			} else {
				http.Redirect(w, r, "/barber/schedule?saved=1", http.StatusSeeOther)
				return
			}
		}
		if status == http.StatusOK {
			status = http.StatusBadRequest
		}
	}
	view := calendarView{Calendar: calendar, CSRF: s.CSRFToken, Error: problem}
	for i, day := range calendar.Week {
		view.Days = append(view.Days, calendarDayView{DayHours: day, Index: i, Name: []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}[i]})
	}
	for _, off := range calendar.TimeOff {
		view.TimeOff += off.Date + " " + off.Start + " " + off.End + "\n"
	}
	if r.Method == http.MethodPost {
		view.TimeOff = r.PostFormValue("time_off")
	}
	renderStatus(w, status, "barber/schedule.html", view)
}
