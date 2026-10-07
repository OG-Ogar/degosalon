// Package geography owns Nigeria's canonical state/LGA choices.
package geography

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed nigeria.json
var catalog embed.FS

type Area struct {
	ID, Name string
	Aliases  []string
}
type State struct {
	ID, Name string
	Aliases  []string
	Areas    []Area
}

var states []State
var stateIndex = map[string]int{}
var areaIndex = map[string]map[string]int{}

func init() {
	data, err := catalog.ReadFile("nigeria.json")
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &states); err != nil {
		panic(err)
	}
	for i, s := range states {
		for _, alias := range append([]string{s.ID, s.Name}, s.Aliases...) {
			stateIndex[key(alias)] = i
		}
		areaIndex[s.ID] = map[string]int{}
		for j, a := range s.Areas {
			for _, alias := range append([]string{a.ID, a.Name}, a.Aliases...) {
				areaIndex[s.ID][key(alias)] = j
			}
		}
	}
}
func key(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.ToLower(text))
}

// States returns a copy so callers cannot change the shared catalog.
func States() []State {
	out := make([]State, len(states))
	for i, s := range states {
		out[i] = s
		out[i].Aliases = append([]string(nil), s.Aliases...)
		out[i].Areas = append([]Area(nil), s.Areas...)
		for j, a := range s.Areas {
			out[i].Areas[j].Aliases = append([]string(nil), a.Aliases...)
		}
	}
	return out
}
func FindState(value string) (State, bool) {
	i, ok := stateIndex[key(value)]
	if !ok {
		return State{}, false
	}
	return states[i], true
}
func FindArea(s State, value string) (Area, bool) {
	i, ok := areaIndex[s.ID][key(value)]
	if !ok {
		return Area{}, false
	}
	return s.Areas[i], true
}
func Resolve(state, area string) (State, Area, bool) {
	s, ok := FindState(state)
	if !ok {
		return State{}, Area{}, false
	}
	a, ok := FindArea(s, area)
	return s, a, ok
}

// NormalizeLegacy preserves an unknown area and never assigns it to an LGA.
func NormalizeLegacy(state, area string) (string, string) {
	s, ok := FindState(state)
	if !ok {
		return state, area
	}
	if a, ok := FindArea(s, area); ok {
		return s.Name, a.Name
	}
	return s.Name, area
}
func StateID(value string) string {
	s, ok := FindState(value)
	if ok {
		return s.ID
	}
	return ""
}
func AreaID(state, area string) string {
	_, a, ok := Resolve(state, area)
	if ok {
		return a.ID
	}
	return ""
}
