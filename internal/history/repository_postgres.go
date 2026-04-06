package history

import (
	"context"
	"database/sql"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) ListRecentByUser(ctx context.Context, userID int64, limit int) ([]Item, error) {
	const query = `
		WITH ranked_events AS (
			SELECT
				pe.track_id,
				pe.event_type,
				pe.position_sec,
				pe.created_at,
				ROW_NUMBER() OVER (PARTITION BY pe.track_id ORDER BY pe.created_at DESC, pe.id DESC) AS rn
			FROM play_events pe
			WHERE pe.user_id = $1
		)
		SELECT
			re.track_id,
			t.title,
			t.artist_name,
			re.created_at,
			re.event_type,
			re.position_sec
		FROM ranked_events re
		JOIN tracks t ON t.id = re.track_id
		WHERE re.rn = 1
		ORDER BY re.created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(
			&item.TrackID,
			&item.Title,
			&item.ArtistName,
			&item.LastPlayedAt,
			&item.LastEventType,
			&item.LastPositionSec,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []Item{}
	}

	return items, nil
}
