package catalog

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"music-stream/internal/platform/store"
)

type PostgresRepository struct{}

func NewPostgresRepository() *PostgresRepository {
	return &PostgresRepository{}
}

func (r *PostgresRepository) Create(ctx context.Context, exec store.DBTX, params CreateTrackParams) (Track, error) {
	const query = `
		INSERT INTO tracks (title, artist_name, album_name, duration_sec, release_date, cover_url, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, title, artist_name, album_name, duration_sec, release_date, cover_url, status, created_at, updated_at
	`

	var (
		albumName   sql.NullString
		releaseDate sql.NullTime
		coverURL    sql.NullString
		track       Track
	)

	err := exec.QueryRowContext(
		ctx,
		query,
		params.Title,
		params.ArtistName,
		ptrStringToNull(params.AlbumName),
		params.DurationSec,
		ptrTimeToNull(params.ReleaseDate),
		ptrStringToNull(params.CoverURL),
		params.Status,
	).Scan(
		&track.ID,
		&track.Title,
		&track.ArtistName,
		&albumName,
		&track.DurationSec,
		&releaseDate,
		&coverURL,
		&track.Status,
		&track.CreatedAt,
		&track.UpdatedAt,
	)
	if err != nil {
		return Track{}, err
	}

	track.AlbumName = nullStringPtr(albumName)
	track.ReleaseDate = nullTimePtr(releaseDate)
	track.CoverURL = nullStringPtr(coverURL)

	return track, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, exec store.DBTX, trackID int64, status string) error {
	const query = `
		UPDATE tracks
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`

	_, err := exec.ExecContext(ctx, query, trackID, status)
	return err
}

func (r *PostgresRepository) ListReady(ctx context.Context, exec store.DBTX, limit int) ([]Track, error) {
	const query = `
		SELECT id, title, artist_name, album_name, duration_sec, release_date, cover_url, status, created_at, updated_at
		FROM tracks
		WHERE status = 'READY'
		ORDER BY release_date DESC NULLS LAST, id DESC
		LIMIT $1
	`

	return scanTracks(ctx, exec, query, limit)
}

func (r *PostgresRepository) FindReadyByID(ctx context.Context, exec store.DBTX, trackID int64) (Track, error) {
	const query = `
		SELECT id, title, artist_name, album_name, duration_sec, release_date, cover_url, status, created_at, updated_at
		FROM tracks
		WHERE id = $1
		  AND status = 'READY'
		LIMIT 1
	`

	items, err := scanTracks(ctx, exec, query, trackID)
	if err != nil {
		return Track{}, err
	}
	if len(items) == 0 {
		return Track{}, ErrNotFound
	}

	return items[0], nil
}

func (r *PostgresRepository) SearchReady(ctx context.Context, exec store.DBTX, queryValue string, limit int) ([]Track, error) {
	const query = `
		SELECT id, title, artist_name, album_name, duration_sec, release_date, cover_url, status, created_at, updated_at
		FROM tracks
		WHERE status = 'READY'
		  AND (
		    title ILIKE '%' || $1 || '%'
		    OR artist_name ILIKE '%' || $1 || '%'
		    OR COALESCE(album_name, '') ILIKE '%' || $1 || '%'
		  )
		ORDER BY GREATEST(
		    similarity(title, $1),
		    similarity(artist_name, $1),
		    similarity(COALESCE(album_name, ''), $1)
		  ) DESC,
		  release_date DESC NULLS LAST,
		  id DESC
		LIMIT $2
	`

	return scanTracks(ctx, exec, query, queryValue, limit)
}

func ptrStringToNull(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: *value, Valid: true}
}

func ptrTimeToNull(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}

	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	v := value.String
	return &v
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	v := value.Time.UTC()
	return &v
}

func scanTracks(ctx context.Context, exec store.DBTX, query string, args ...any) ([]Track, error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var (
			albumName   sql.NullString
			releaseDate sql.NullTime
			coverURL    sql.NullString
			track       Track
		)

		if err := rows.Scan(
			&track.ID,
			&track.Title,
			&track.ArtistName,
			&albumName,
			&track.DurationSec,
			&releaseDate,
			&coverURL,
			&track.Status,
			&track.CreatedAt,
			&track.UpdatedAt,
		); err != nil {
			return nil, err
		}

		track.AlbumName = nullStringPtr(albumName)
		track.ReleaseDate = nullTimePtr(releaseDate)
		track.CoverURL = nullStringPtr(coverURL)
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tracks == nil {
		tracks = []Track{}
	}

	return tracks, nil
}

var ErrNotFound = errors.New("catalog not found")
