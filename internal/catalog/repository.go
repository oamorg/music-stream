package catalog

import (
	"context"

	"music-stream/internal/platform/store"
)

type Repository interface {
	Create(ctx context.Context, exec store.DBTX, params CreateTrackParams) (Track, error)
	UpdateStatus(ctx context.Context, exec store.DBTX, trackID int64, status string) error
	ListReady(ctx context.Context, exec store.DBTX, limit int) ([]Track, error)
	FindReadyByID(ctx context.Context, exec store.DBTX, trackID int64) (Track, error)
	SearchReady(ctx context.Context, exec store.DBTX, query string, limit int) ([]Track, error)
}
