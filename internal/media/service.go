package media

import (
	"context"
	"database/sql"
)

type LookupService struct {
	db     *sql.DB
	assets TrackAssetRepository
}

func NewLookupService(db *sql.DB, assets TrackAssetRepository) *LookupService {
	return &LookupService{
		db:     db,
		assets: assets,
	}
}

func (s *LookupService) FindReadyAssetByTrackID(ctx context.Context, trackID int64) (TrackAsset, error) {
	return s.assets.FindReadyByTrackID(ctx, s.db, trackID)
}
