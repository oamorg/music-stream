package playback

import "time"

const (
	AccessTypeStream   = "STREAM"
	EventTypeStart     = "START"
	EventTypeHeartbeat = "HEARTBEAT"
	EventTypeComplete  = "COMPLETE"
)

type UserEntitlement struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"userId"`
	TrackID    int64      `json:"trackId"`
	AccessType string     `json:"accessType"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type PlaybackSession struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"userId"`
	TrackID     int64     `json:"trackId"`
	AssetID     int64     `json:"assetId"`
	ManifestURL string    `json:"manifestUrl"`
	ExpiresAt   time.Time `json:"expiresAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

type PlayEvent struct {
	ID              int64     `json:"id"`
	SessionID       int64     `json:"sessionId"`
	UserID          int64     `json:"userId"`
	TrackID         int64     `json:"trackId"`
	EventType       string    `json:"eventType"`
	PositionSec     int       `json:"positionSec"`
	ClientTimestamp time.Time `json:"clientTimestamp"`
	ServerTimestamp time.Time `json:"serverTimestamp"`
	CreatedAt       time.Time `json:"createdAt"`
}

type CreateSessionInput struct {
	TrackID int64 `json:"trackId"`
}

type ReportEventInput struct {
	SessionID       int64     `json:"sessionId"`
	EventType       string    `json:"eventType"`
	PositionSec     int       `json:"positionSec"`
	ClientTimestamp time.Time `json:"clientTimestamp"`
}
