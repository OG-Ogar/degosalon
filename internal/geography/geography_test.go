package geography

import "testing"

func TestCatalog(t *testing.T) {
	seen := map[string]bool{}
	count := 0
	if len(states) != 37 {
		t.Fatalf("states/FCT=%d", len(states))
	}
	for _, s := range states {
		if seen[s.ID] {
			t.Fatal("duplicate state", s.ID)
		}
		seen[s.ID] = true
		names := map[string]bool{}
		for _, a := range s.Areas {
			count++
			if seen[a.ID] || names[key(a.Name)] {
				t.Fatal("duplicate area", s.ID, a.ID)
			}
			seen[a.ID] = true
			names[key(a.Name)] = true
			for _, v := range append([]string{a.Name, a.ID}, a.Aliases...) {
				_, got, ok := Resolve(s.ID, v)
				if !ok || got.ID != a.ID {
					t.Fatalf("unresolved alias %s/%s", s.ID, v)
				}
			}
		}
	}
	if count != 774 {
		t.Fatalf("areas=%d", count)
	}
	for _, tc := range []struct{ state, area, name string }{{" Lagos STATE ", " i-keja ", "Ikeja"}, {"Abuja", "Abuja Municipal", "Abuja Municipal"}, {"Ebonyi", "Afikpo South", "Edda"}, {"Kano", "Kunchi", "Ghari"}, {"Ogun", "Egbado North", "Yewa North"}} {
		_, a, ok := Resolve(tc.state, tc.area)
		if !ok || a.Name != tc.name {
			t.Fatalf("%+v: %+v %v", tc, a, ok)
		}
	}
	if _, _, ok := Resolve("Lagos", "Abuja Municipal"); ok {
		t.Fatal("cross-state area accepted")
	}
	if _, _, ok := Resolve("Lagos", "Victoria Island"); ok {
		t.Fatal("neighborhood guessed")
	}
	s, a := NormalizeLegacy("LAGOS", "Unknown neighborhood")
	if s != "Lagos" || a != "Unknown neighborhood" {
		t.Fatal("legacy data lost")
	}
}
