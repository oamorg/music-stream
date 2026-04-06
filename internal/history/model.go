package history

import "time"

type Item struct {
	TrackID         int64     `json:"trackId"`
	Title           string    `json:"title"`
	ArtistName      string    `json:"artistName"`
	LastPlayedAt    time.Time `json:"lastPlayedAt"`
	LastEventType   string    `json:"lastEventType"`
	LastPositionSec int       `json:"lastPositionSec"`
}
