package handlers

import "testing"

// TestBookingTimeWindow pins the boundaries of the bookable day. The
// closing time is included on purpose: the form offers it as its max, and
// a value the browser accepts must not be refused by the server.
func TestBookingTimeWindow(t *testing.T) {
	cases := []struct {
		value  string
		want   string
		accept bool
	}{
		{"08:00", "08:00", true}, // opening, the first slot
		{"8:00", "08:00", true},  // unpadded, normalised
		{"13:45", "13:45", true}, // the middle of the day
		{"19:59", "19:59", true}, // a minute before closing
		{"20:00", "20:00", true}, // closing, the last slot
		{"20:01", "", false},     // past closing
		{"07:59", "", false},     // before opening
		{"", "", false},          // nothing chosen
		{"half past", "", false}, // not a time at all
		{"8:00 PM", "", false},   // a time input never submits this
	}

	for _, c := range cases {
		slot, problem := validBookingTime(c.value)

		if c.accept && problem != "" {
			t.Errorf("validBookingTime(%q) refused it: %s", c.value, problem)
			continue
		}

		if !c.accept && problem == "" {
			t.Errorf("validBookingTime(%q) was accepted, want a refusal", c.value)
			continue
		}

		if slot != c.want {
			t.Errorf("validBookingTime(%q) = %q, want %q", c.value, slot, c.want)
		}
	}
}

// TestBookingHoursFromEnvironment covers the shop that does not keep the
// default hours, including the values that have to be refused rather than
// quietly turned into a window nobody can book in.
func TestBookingHoursFromEnvironment(t *testing.T) {
	// The window is package state, so it is put back afterwards.
	openWas, closeWas := openingHour, closingHour
	t.Cleanup(func() { openingHour, closingHour = openWas, closeWas })

	t.Setenv("BOOKING_OPEN", "6")
	t.Setenv("BOOKING_CLOSE", "22")

	if err := InitBookingHours(); err != nil {
		t.Fatalf("InitBookingHours: %v", err)
	}

	if opening, closing := BookingHours(); opening != "06:00" || closing != "22:00" {
		t.Fatalf("hours are %s to %s, want 06:00 to 22:00", opening, closing)
	}

	if _, problem := validBookingTime("21:30"); problem != "" {
		t.Errorf("21:30 was refused inside a 06:00-22:00 day: %s", problem)
	}

	for _, bad := range []string{"25", "-1", "eight"} {
		t.Setenv("BOOKING_CLOSE", bad)

		if err := InitBookingHours(); err == nil {
			t.Errorf("BOOKING_CLOSE=%q was accepted", bad)
		}
	}

	// A window that ends before it starts leaves nothing bookable.
	t.Setenv("BOOKING_OPEN", "20")
	t.Setenv("BOOKING_CLOSE", "8")

	if err := InitBookingHours(); err == nil {
		t.Error("an inverted window was accepted")
	}
}
