package models

// SocialMusicTrack describes a soundtrack record. The post composer no longer
// offers soundtracks, but the catalogue and its validation stay in place for
// older stored posts and for a future re-enable once licensed audio exists.
//
// Copyrighted recordings are not bundled with the app. See
// static/music/README.md for the expected filenames.
type SocialMusicTrack struct {
	ID       string
	Name     string
	Mood     string
	Duration string
	URL      string
}

var socialMusicTracks = []SocialMusicTrack{
	{ID: "mafia-soothewave", Name: "Mafia Soothewave", Mood: "Soothewave Naija TikTok trend", URL: "/static/music/mafia-soothewave.mp3"},
	{ID: "team-trump", Name: "Team Trump", Mood: "Viral meme sound", URL: "/static/music/team-trump.mp3"},
	{ID: "when-my-god-say-yes", Name: "When My God Say Yes", Mood: "Gospel TikTok trend", URL: "/static/music/when-my-god-say-yes.mp3"},
	{ID: "aura-farming", Name: "Aura Farming", Mood: "Viral aura trend", URL: "/static/music/aura-farming.mp3"},
	{ID: "forever-young", Name: "Forever Young", Mood: "Nostalgic pop trend", URL: "/static/music/forever-young.mp3"},
	{ID: "smooth-criminal", Name: "Smooth Criminal", Mood: "Michael Jackson dance trend", URL: "/static/music/smooth-criminal.mp3"},
	{ID: "plutomania", Name: "PLUTOMANIA", Mood: "Hyperpop TikTok trend", URL: "/static/music/plutomania.mp3"},
	{ID: "i-gat-black-i-gat-white", Name: "I Gat Black, I Gat White", Mood: "Viral Naija comedy sound", URL: "/static/music/i-gat-black-i-gat-white.mp3"},
	{ID: "junior", Name: "Junior", Mood: "Naija street anthem", URL: "/static/music/junior.mp3"},
	{ID: "let-me-be", Name: "LET ME BE", Mood: "Afrobeats TikTok trend", URL: "/static/music/let-me-be.mp3"},
	{ID: "akons-beautiful-day", Name: "Akon's Beautiful Day", Mood: "Akon feel-good trend", URL: "/static/music/akons-beautiful-day.mp3"},
	{ID: "never-give-up", Name: "Never Give Up", Mood: "Motivational TikTok trend", URL: "/static/music/never-give-up.mp3"},
	{ID: "last-last", Name: "Last Last", Mood: "Burna Boy Afrobeats", URL: "/static/music/last-last.mp3"},
	{ID: "rush", Name: "Rush", Mood: "Ayra Starr Afrobeats", URL: "/static/music/rush.mp3"},
	{ID: "sungba", Name: "Sungba", Mood: "Asake amapiano", URL: "/static/music/sungba.mp3"},
	{ID: "essence", Name: "Essence", Mood: "Wizkid Afrobeats", URL: "/static/music/essence.mp3"},
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
