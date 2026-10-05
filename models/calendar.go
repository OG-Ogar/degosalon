package models

type DayHours struct {
	Closed               bool
	Open, Close          string
	BreakStart, BreakEnd string
}

type TimeOff struct{ Date, Start, End string }

type Calendar struct {
	Timezone                 string
	LeadMinutes, HorizonDays int
	Week                     [7]DayHours
	TimeOff                  []TimeOff
}
