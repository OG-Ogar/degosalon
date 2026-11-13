package models

import "testing"

func TestSocialMusicCatalogueIsConsistent(t *testing.T) {
	seen := make(map[string]bool)
	for _, track := range SocialMusicTracks() {
		if track.ID == "" || track.Name == "" {
			t.Fatalf("track has an empty ID or name: %+v", track)
		}
		if seen[track.ID] {
			t.Fatalf("duplicate soundtrack ID %q", track.ID)
		}
		seen[track.ID] = true

		wantURL := "/static/music/" + track.ID + ".mp3"
		if track.URL != wantURL {
			t.Fatalf("track %q URL = %q, want %q", track.ID, track.URL, wantURL)
		}
	}
}

func TestLegacySoundtrackIDsAreNoLongerValid(t *testing.T) {
	for _, id := range []string{"bounce-pop", "phonk-drift", "smooth-rnb"} {
		if ValidSocialMusicID(id) {
			t.Fatalf("legacy soundtrack %q should no longer be valid", id)
		}
	}
}
