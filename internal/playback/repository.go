package playback

import (
	"context"
	"time"
)

type Repository interface {
	FindActiveEntitlement(ctx context.Context, userID, trackID int64, now time.Time) (UserEntitlement, error)
	CreateSession(ctx context.Context, userID, trackID, assetID int64, manifestURL string, expiresAt time.Time) (PlaybackSession, error)
	FindSessionByIDAndUser(ctx context.Context, sessionID, userID int64) (PlaybackSession, error)
	CreateEvent(ctx context.Context, input ReportEventInput, userID, trackID int64) (PlayEvent, error)
}
