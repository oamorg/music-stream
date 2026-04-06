package media

import (
	"context"
	"database/sql"
	"errors"

	"music-stream/internal/platform/store"
)

type PostgresTrackAssetRepository struct{}

func NewPostgresTrackAssetRepository() *PostgresTrackAssetRepository {
	return &PostgresTrackAssetRepository{}
}

func (r *PostgresTrackAssetRepository) Create(ctx context.Context, exec store.DBTX, params CreateTrackAssetParams) (TrackAsset, error) {
	const query = `
		INSERT INTO track_assets (track_id, source_object_key, hls_manifest_key, audio_codec, bitrate_kbps, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, track_id, source_object_key, hls_manifest_key, audio_codec, bitrate_kbps, status, error_message, created_at, updated_at
	`

	var (
		hlsManifestKey sql.NullString
		audioCodec     sql.NullString
		bitrateKbps    sql.NullInt64
		errorMessage   sql.NullString
		asset          TrackAsset
	)

	err := exec.QueryRowContext(
		ctx,
		query,
		params.TrackID,
		params.SourceObjectKey,
		ptrStringToNull(params.HLSManifestKey),
		ptrStringToNull(params.AudioCodec),
		ptrIntToNull(params.BitrateKbps),
		params.Status,
		ptrStringToNull(params.ErrorMessage),
	).Scan(
		&asset.ID,
		&asset.TrackID,
		&asset.SourceObjectKey,
		&hlsManifestKey,
		&audioCodec,
		&bitrateKbps,
		&asset.Status,
		&errorMessage,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	)
	if err != nil {
		return TrackAsset{}, err
	}

	asset.HLSManifestKey = nullStringPtr(hlsManifestKey)
	asset.AudioCodec = nullStringPtr(audioCodec)
	asset.BitrateKbps = nullIntPtr(bitrateKbps)
	asset.ErrorMessage = nullStringPtr(errorMessage)

	return asset, nil
}

func (r *PostgresTrackAssetRepository) UpdateStatus(ctx context.Context, exec store.DBTX, assetID int64, status string, errorMessage *string) error {
	const query = `
		UPDATE track_assets
		SET status = $2, error_message = $3, updated_at = NOW()
		WHERE id = $1
	`

	_, err := exec.ExecContext(ctx, query, assetID, status, ptrStringToNull(errorMessage))
	return err
}

func (r *PostgresTrackAssetRepository) MarkReady(ctx context.Context, exec store.DBTX, assetID int64, manifestKey, audioCodec string, bitrateKbps int) error {
	const query = `
		UPDATE track_assets
		SET status = 'READY',
		    hls_manifest_key = $2,
		    audio_codec = $3,
		    bitrate_kbps = $4,
		    error_message = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := exec.ExecContext(ctx, query, assetID, manifestKey, audioCodec, bitrateKbps)
	return err
}

func (r *PostgresTrackAssetRepository) MarkFailed(ctx context.Context, exec store.DBTX, assetID int64, errorMessage string) error {
	const query = `
		UPDATE track_assets
		SET status = 'FAILED',
		    error_message = $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := exec.ExecContext(ctx, query, assetID, errorMessage)
	return err
}

func (r *PostgresTrackAssetRepository) FindReadyByTrackID(ctx context.Context, exec store.DBTX, trackID int64) (TrackAsset, error) {
	const query = `
		SELECT id, track_id, source_object_key, hls_manifest_key, audio_codec, bitrate_kbps, status, error_message, created_at, updated_at
		FROM track_assets
		WHERE track_id = $1
		  AND status = 'READY'
		ORDER BY id DESC
		LIMIT 1
	`

	var (
		hlsManifestKey sql.NullString
		audioCodec     sql.NullString
		bitrateKbps    sql.NullInt64
		errorMessage   sql.NullString
		asset          TrackAsset
	)

	err := exec.QueryRowContext(ctx, query, trackID).Scan(
		&asset.ID,
		&asset.TrackID,
		&asset.SourceObjectKey,
		&hlsManifestKey,
		&audioCodec,
		&bitrateKbps,
		&asset.Status,
		&errorMessage,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TrackAsset{}, ErrNotFound
		}
		return TrackAsset{}, err
	}

	asset.HLSManifestKey = nullStringPtr(hlsManifestKey)
	asset.AudioCodec = nullStringPtr(audioCodec)
	asset.BitrateKbps = nullIntPtr(bitrateKbps)
	asset.ErrorMessage = nullStringPtr(errorMessage)

	return asset, nil
}

func ptrStringToNull(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: *value, Valid: true}
}

func ptrIntToNull(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	v := value.String
	return &v
}

func nullIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}

	v := int(value.Int64)
	return &v
}

var ErrNotFound = errors.New("media not found")
