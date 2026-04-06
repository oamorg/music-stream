package media

import (
	"time"

	"music-stream/internal/catalog"
	"music-stream/internal/platform/store"
)

const (
	TrackAssetStatusUploaded    = "UPLOADED"
	TrackAssetStatusTranscoding = "TRANSCODING"
	TrackAssetStatusReady       = "READY"
	TrackAssetStatusFailed      = "FAILED"

	EventTypeTrackTranscodeRequested = "track.transcode.requested"
	AggregateTypeTrackAsset          = "track_asset"
)

type TrackAsset struct {
	ID              int64      `json:"id"`
	TrackID         int64      `json:"trackId"`
	SourceObjectKey string     `json:"sourceObjectKey"`
	HLSManifestKey  *string    `json:"hlsManifestKey,omitempty"`
	AudioCodec      *string    `json:"audioCodec,omitempty"`
	BitrateKbps     *int       `json:"bitrateKbps,omitempty"`
	Status          string     `json:"status"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type CreateTrackAssetParams struct {
	TrackID         int64
	SourceObjectKey string
	HLSManifestKey  *string
	AudioCodec      *string
	BitrateKbps     *int
	Status          string
	ErrorMessage    *string
}

type ImportTrackInput struct {
	Title           string
	ArtistName      string
	AlbumName       *string
	DurationSec     int
	ReleaseDate     *time.Time
	CoverURL        *string
	SourceObjectKey string
}

type TranscodeRequestedPayload struct {
	TrackID         int64     `json:"trackId"`
	AssetID         int64     `json:"assetId"`
	SourceObjectKey string    `json:"sourceObjectKey"`
	RequestedAt     time.Time `json:"requestedAt"`
	TargetFormat    string    `json:"targetFormat"`
}

type ImportTrackResult struct {
	Track  catalog.Track     `json:"track"`
	Asset  TrackAsset        `json:"asset"`
	Outbox store.OutboxEvent `json:"outbox"`
}
