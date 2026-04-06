package media

import (
	"context"

	"music-stream/internal/platform/store"
)

type TrackAssetRepository interface {
	Create(ctx context.Context, exec store.DBTX, params CreateTrackAssetParams) (TrackAsset, error)
	UpdateStatus(ctx context.Context, exec store.DBTX, assetID int64, status string, errorMessage *string) error
	FindReadyByTrackID(ctx context.Context, exec store.DBTX, trackID int64) (TrackAsset, error)
	MarkReady(ctx context.Context, exec store.DBTX, assetID int64, manifestKey, audioCodec string, bitrateKbps int) error
	MarkFailed(ctx context.Context, exec store.DBTX, assetID int64, errorMessage string) error
}
