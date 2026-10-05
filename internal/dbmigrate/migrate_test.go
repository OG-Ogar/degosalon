package dbmigrate

import "testing"

func TestMigrationHistoryValidation(t *testing.T) {
	all := []migration{{version: "001", checksum: "first"}, {version: "002", checksum: "second"}}
	for _, tc := range []struct {
		name    string
		history map[string]string
		valid   bool
	}{
		{"fresh", map[string]string{}, true},
		{"pending", map[string]string{"001": "first"}, true},
		{"current", map[string]string{"001": "first", "002": "second"}, true},
		{"edited", map[string]string{"001": "changed"}, false},
		{"unknown", map[string]string{"003": "third"}, false},
		{"out of order", map[string]string{"002": "second"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validate(all, tc.history); (err == nil) != tc.valid {
				t.Fatalf("validation error=%v valid=%v", err, tc.valid)
			}
		})
	}
}
