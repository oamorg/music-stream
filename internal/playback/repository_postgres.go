package playback

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) FindActiveEntitlement(ctx context.Context, userID, trackID int64, now time.Time) (UserEntitlement, error) {
	const query = `
		SELECT id, user_id, track_id, access_type, expires_at, created_at
		FROM user_entitlements
		WHERE user_id = $1
		  AND track_id = $2
		  AND (expires_at IS NULL OR expires_at > $3)
		LIMIT 1
	`

	var (
		entitlement UserEntitlement
		expiresAt   sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, query, userID, trackID, now.UTC()).Scan(
		&entitlement.ID,
		&entitlement.UserID,
		&entitlement.TrackID,
		&entitlement.AccessType,
		&expiresAt,
		&entitlement.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserEntitlement{}, ErrNotFound
		}
		return UserEntitlement{}, err
	}

	if expiresAt.Valid {
		value := expiresAt.Time.UTC()
		entitlement.ExpiresAt = &value
	}

	return entitlement, nil
}

func (r *PostgresRepository) CreateSession(ctx context.Context, userID, trackID, assetID int64, manifestURL string, expiresAt time.Time) (PlaybackSession, error) {
	const query = `
		INSERT INTO playback_sessions (user_id, track_id, asset_id, manifest_url, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, track_id, asset_id, manifest_url, expires_at, created_at
	`

	var session PlaybackSession
	err := r.db.QueryRowContext(ctx, query, userID, trackID, assetID, manifestURL, expiresAt.UTC()).Scan(
		&session.ID,
		&session.UserID,
		&session.TrackID,
		&session.AssetID,
		&session.ManifestURL,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if err != nil {
		return PlaybackSession{}, err
	}

	return session, nil
}

func (r *PostgresRepository) FindSessionByIDAndUser(ctx context.Context, sessionID, userID int64) (PlaybackSession, error) {
	const query = `
		SELECT id, user_id, track_id, asset_id, manifest_url, expires_at, created_at
		FROM playback_sessions
		WHERE id = $1
		  AND user_id = $2
		LIMIT 1
	`

	var session PlaybackSession
	err := r.db.QueryRowContext(ctx, query, sessionID, userID).Scan(
		&session.ID,
		&session.UserID,
		&session.TrackID,
		&session.AssetID,
		&session.ManifestURL,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlaybackSession{}, ErrNotFound
		}
		return PlaybackSession{}, err
	}

	return session, nil
}

func (r *PostgresRepository) CreateEvent(ctx context.Context, input ReportEventInput, userID, trackID int64) (PlayEvent, error) {
	const query = `
		INSERT INTO play_events (session_id, user_id, track_id, event_type, position_sec, client_timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, session_id, user_id, track_id, event_type, position_sec, client_timestamp, server_timestamp, created_at
	`

	var event PlayEvent
	err := r.db.QueryRowContext(
		ctx,
		query,
		input.SessionID,
		userID,
		trackID,
		input.EventType,
		input.PositionSec,
		input.ClientTimestamp.UTC(),
	).Scan(
		&event.ID,
		&event.SessionID,
		&event.UserID,
		&event.TrackID,
		&event.EventType,
		&event.PositionSec,
		&event.ClientTimestamp,
		&event.ServerTimestamp,
		&event.CreatedAt,
	)
	if err != nil {
		return PlayEvent{}, err
	}

	return event, nil
}

var ErrNotFound = errors.New("playback not found")
