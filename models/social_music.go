package models

// SocialMusicTrack is one short, loopable soundtrack a member can attach to a
// photo or video post. Every track is an original, royalty-free production
// written in a style popular on short-video feeds (dance-pop, phonk, Afrobeats,
// amapiano, Jersey club, hyperpop, lo-fi, cinematic pop, R&B, tech house and
// indie). IDs are stored with the post; URLs point at the bundled audio files,
// so no upload is needed for a soundtrack.
type SocialMusicTrack struct {
	ID       string
	Name     string
	Mood     string
	Duration string
	URL      string
}

var socialMusicTracks = []SocialMusicTrack{
	{ID: "bounce-pop", Name: "Bounce Pop", Mood: "Upbeat dance-pop", Duration: "0:15", URL: "/static/music/bounce-pop.mp3"},
	{ID: "phonk-drift", Name: "Phonk Drift", Mood: "Dark drift phonk", Duration: "0:14", URL: "/static/music/phonk-drift.mp3"},
	{ID: "sped-up-pop", Name: "Sped-Up Sparkle", Mood: "Nightcore pop", Duration: "0:12", URL: "/static/music/sped-up-pop.mp3"},
	{ID: "afro-bounce", Name: "Afro Bounce", Mood: "Afrobeats groove", Duration: "0:18", URL: "/static/music/afro-bounce.mp3"},
	{ID: "amapiano-sunset", Name: "Amapiano Sunset", Mood: "Log-drum amapiano", Duration: "0:17", URL: "/static/music/amapiano-sunset.mp3"},
	{ID: "jersey-club", Name: "Jersey Club", Mood: "Bouncy club energy", Duration: "0:14", URL: "/static/music/jersey-club.mp3"},
	{ID: "hyperpop-rush", Name: "Hyperpop Rush", Mood: "Glitchy hyperpop", Duration: "0:12", URL: "/static/music/hyperpop-rush.mp3"},
	{ID: "lofi-aesthetic", Name: "Lo-Fi Aesthetic", Mood: "Chill study beats", Duration: "0:12", URL: "/static/music/lofi-aesthetic.mp3"},
	{ID: "main-character", Name: "Main Character", Mood: "Cinematic pop", Duration: "0:21", URL: "/static/music/main-character.mp3"},
	{ID: "smooth-rnb", Name: "Smooth R&B", Mood: "Late-night R&B", Duration: "0:22", URL: "/static/music/smooth-rnb.mp3"},
	{ID: "tech-house", Name: "Tech House", Mood: "Deep club groove", Duration: "0:15", URL: "/static/music/tech-house.mp3"},
	{ID: "indie-summer", Name: "Indie Summer", Mood: "Jangly indie pop", Duration: "0:16", URL: "/static/music/indie-summer.mp3"},
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
