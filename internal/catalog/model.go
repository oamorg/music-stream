package catalog

import "time"

const (
	TrackStatusDraft      = "DRAFT"
	TrackStatusProcessing = "PROCESSING"
	TrackStatusReady      = "READY"
	TrackStatusBlocked    = "BLOCKED"
)

type Track struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	ArtistName  string     `json:"artistName"`
	AlbumName   *string    `json:"albumName,omitempty"`
	DurationSec int        `json:"durationSec"`
	ReleaseDate *time.Time `json:"releaseDate,omitempty"`
	CoverURL    *string    `json:"coverUrl,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type CreateTrackParams struct {
	Title       string
	ArtistName  string
	AlbumName   *string
	DurationSec int
	ReleaseDate *time.Time
	CoverURL    *string
	Status      string
}
