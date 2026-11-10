package models

// SocialMusicTrack is one short, loopable soundtrack a member can attach to a
// photo or video post. IDs are stored with the post; URLs point at the bundled
// audio files, so no upload is needed for a soundtrack.
type SocialMusicTrack struct {
	ID       string
	Name     string
	Mood     string
	Duration string
	URL      string
}

var socialMusicTracks = []SocialMusicTrack{
	{ID: "sunrise-glow", Name: "Sunrise Glow", Mood: "Bright & happy", Duration: "0:10", URL: "/static/music/sunrise-glow.wav"},
	{ID: "velvet-groove", Name: "Velvet Groove", Mood: "Mellow R&B", Duration: "0:11", URL: "/static/music/velvet-groove.wav"},
	{ID: "soft-focus", Name: "Soft Focus", Mood: "Calm & airy", Duration: "0:13", URL: "/static/music/soft-focus.wav"},
	{ID: "golden-hour", Name: "Golden Hour", Mood: "Warm & hopeful", Duration: "0:11", URL: "/static/music/golden-hour.wav"},
	{ID: "fresh-fade", Name: "Fresh Fade", Mood: "Upbeat salon energy", Duration: "0:09", URL: "/static/music/fresh-fade.wav"},
	{ID: "midnight-sheen", Name: "Midnight Sheen", Mood: "Smooth after-hours", Duration: "0:12", URL: "/static/music/midnight-sheen.wav"},
}

// SocialMusicTracks returns the soundtrack menu shown in the post composer.
func SocialMusicTracks() []SocialMusicTrack {
	return append([]SocialMusicTrack(nil), socialMusicTracks...)
}

// SocialMusicTrackByID finds a bundled soundtrack by its stored ID.
func SocialMusicTrackByID(id string) (SocialMusicTrack, bool) {
	for _, track := range socialMusicTracks {
		if track.ID == id {
			return track, true
		}
	}
	return SocialMusicTrack{}, false
}

// ValidSocialMusicID reports whether id names a bundled soundtrack. An empty
// ID is valid and means the post has no music.
func ValidSocialMusicID(id string) bool {
	if id == "" {
		return true
	}
	_, ok := SocialMusicTrackByID(id)
	return ok
}
