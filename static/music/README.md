# Soundtrack audio files

The "Add sound" picker in the post composer is driven by
[`models/social_music.go`](../../models/social_music.go). The catalogue now lists
trending Nigerian TikTok songs, but **no copyrighted recordings are bundled with
this repository**.

To enable playback, add a licensed MP3 for each track using the exact filenames
below, then commit the files:

| Track | Filename |
| --- | --- |
| Mafia Soothewave | `mafia-soothewave.mp3` |
| Team Trump | `team-trump.mp3` |
| When My God Say Yes | `when-my-god-say-yes.mp3` |
| Aura Farming | `aura-farming.mp3` |
| Forever Young | `forever-young.mp3` |
| Smooth Criminal | `smooth-criminal.mp3` |
| PLUTOMANIA | `plutomania.mp3` |
| I Gat Black, I Gat White | `i-gat-black-i-gat-white.mp3` |
| Junior | `junior.mp3` |
| LET ME BE | `let-me-be.mp3` |
| Akon's Beautiful Day | `akons-beautiful-day.mp3` |
| Never Give Up | `never-give-up.mp3` |
| Last Last | `last-last.mp3` |
| Rush | `rush.mp3` |
| Sungba | `sungba.mp3` |
| Essence | `essence.mp3` |

## Licensing

Only add audio you have the right to distribute — for example, a signed licence
from the rights holder, a royalty-free cover/version, or a track you produced.
Do not rip audio from YouTube, TikTok, Spotify or other streaming services.

Until a file is added, the picker still lists the track, but previewing or
playing it will show an "audio not added" message instead of breaking the page.

## Adding or removing tracks

1. Add the MP3 here.
2. Add a `SocialMusicTrack` entry with the same slug in
   `models/social_music.go`.
3. `Duration` is optional; leave it empty to hide the duration in the picker.
