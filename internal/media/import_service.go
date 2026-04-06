package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"music-stream/internal/catalog"
	"music-stream/internal/platform/store"
)

var (
	ErrInvalidTrackTitle     = errors.New("invalid track title")
	ErrInvalidArtistName     = errors.New("invalid artist name")
	ErrInvalidDuration       = errors.New("invalid duration")
	ErrInvalidSourceObjectKey = errors.New("invalid source object key")
)

type ImportService struct {
	db      *sql.DB
	tracks  catalog.Repository
	assets  TrackAssetRepository
	outbox  store.OutboxRepository
	now     func() time.Time
}

func NewImportService(
	db *sql.DB,
	tracks catalog.Repository,
	assets TrackAssetRepository,
	outbox store.OutboxRepository,
	now func() time.Time,
) *ImportService {
	if now == nil {
		now = time.Now
	}

	return &ImportService{
		db:     db,
		tracks: tracks,
		assets: assets,
		outbox: outbox,
		now:    now,
	}
}

func (s *ImportService) ImportTrack(ctx context.Context, input ImportTrackInput) (ImportTrackResult, error) {
	normalized, err := normalizeImportTrackInput(input)
	if err != nil {
		return ImportTrackResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportTrackResult{}, err
	}
	defer tx.Rollback()

	track, err := s.tracks.Create(ctx, tx, catalog.CreateTrackParams{
		Title:       normalized.Title,
		ArtistName:  normalized.ArtistName,
		AlbumName:   normalized.AlbumName,
		DurationSec: normalized.DurationSec,
		ReleaseDate: normalized.ReleaseDate,
		CoverURL:    normalized.CoverURL,
		Status:      catalog.TrackStatusDraft,
	})
	if err != nil {
		return ImportTrackResult{}, err
	}

	asset, err := s.assets.Create(ctx, tx, CreateTrackAssetParams{
		TrackID:         track.ID,
		SourceObjectKey: normalized.SourceObjectKey,
		Status:          TrackAssetStatusUploaded,
	})
	if err != nil {
		return ImportTrackResult{}, err
	}

	payload, err := json.Marshal(TranscodeRequestedPayload{
		TrackID:         track.ID,
		AssetID:         asset.ID,
		SourceObjectKey: asset.SourceObjectKey,
		RequestedAt:     s.now().UTC(),
		TargetFormat:    "HLS",
	})
	if err != nil {
		return ImportTrackResult{}, err
	}

	outboxEvent, err := s.outbox.Create(ctx, tx, store.CreateOutboxEventParams{
		EventType:     EventTypeTrackTranscodeRequested,
		AggregateType: AggregateTypeTrackAsset,
		AggregateID:   strconv.FormatInt(asset.ID, 10),
		Payload:       payload,
		Status:        store.OutboxStatusPending,
	})
	if err != nil {
		return ImportTrackResult{}, err
	}

	if err := s.tracks.UpdateStatus(ctx, tx, track.ID, catalog.TrackStatusProcessing); err != nil {
		return ImportTrackResult{}, err
	}
	track.Status = catalog.TrackStatusProcessing

	if err := tx.Commit(); err != nil {
		return ImportTrackResult{}, err
	}

	return ImportTrackResult{
		Track:  track,
		Asset:  asset,
		Outbox: outboxEvent,
	}, nil
}

func normalizeImportTrackInput(input ImportTrackInput) (ImportTrackInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.ArtistName = strings.TrimSpace(input.ArtistName)
	input.SourceObjectKey = strings.TrimSpace(input.SourceObjectKey)

	if input.Title == "" {
		return ImportTrackInput{}, ErrInvalidTrackTitle
	}
	if input.ArtistName == "" {
		return ImportTrackInput{}, ErrInvalidArtistName
	}
	if input.DurationSec < 0 {
		return ImportTrackInput{}, ErrInvalidDuration
	}
	if input.SourceObjectKey == "" {
		return ImportTrackInput{}, ErrInvalidSourceObjectKey
	}

	if input.AlbumName != nil {
		trimmed := strings.TrimSpace(*input.AlbumName)
		if trimmed == "" {
			input.AlbumName = nil
		} else {
			input.AlbumName = &trimmed
		}
	}

	if input.CoverURL != nil {
		trimmed := strings.TrimSpace(*input.CoverURL)
		if trimmed == "" {
			input.CoverURL = nil
		} else {
			input.CoverURL = &trimmed
		}
	}

	if input.ReleaseDate != nil {
		releaseDate := input.ReleaseDate.UTC()
		input.ReleaseDate = &releaseDate
	}

	return input, nil
}
