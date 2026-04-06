package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const (
	OutboxStatusPending    = "PENDING"
	OutboxStatusProcessing = "PROCESSING"
	OutboxStatusProcessed  = "PROCESSED"
	OutboxStatusFailed     = "FAILED"
)

var ErrNoPendingOutboxEvents = errors.New("no pending outbox events")

type OutboxEvent struct {
	ID            int64           `json:"id"`
	EventType     string          `json:"eventType"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   string          `json:"aggregateId"`
	Payload       json.RawMessage `json:"payload"`
	Status        string          `json:"status"`
	CreatedAt     time.Time       `json:"createdAt"`
	ProcessedAt   *time.Time      `json:"processedAt,omitempty"`
}

type CreateOutboxEventParams struct {
	EventType     string
	AggregateType string
	AggregateID   string
	Payload       json.RawMessage
	Status        string
}

type OutboxRepository interface {
	Create(ctx context.Context, exec DBTX, params CreateOutboxEventParams) (OutboxEvent, error)
}

type PostgresOutboxRepository struct {
	db *sql.DB
}

func NewPostgresOutboxRepository(db *sql.DB) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{db: db}
}

func (r *PostgresOutboxRepository) Create(ctx context.Context, exec DBTX, params CreateOutboxEventParams) (OutboxEvent, error) {
	const query = `
		INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, event_type, aggregate_type, aggregate_id, payload, status, created_at, processed_at
	`

	status := params.Status
	if status == "" {
		status = OutboxStatusPending
	}

	var event OutboxEvent
	err := exec.QueryRowContext(
		ctx,
		query,
		params.EventType,
		params.AggregateType,
		params.AggregateID,
		[]byte(params.Payload),
		status,
	).Scan(
		&event.ID,
		&event.EventType,
		&event.AggregateType,
		&event.AggregateID,
		&event.Payload,
		&event.Status,
		&event.CreatedAt,
		&event.ProcessedAt,
	)
	if err != nil {
		return OutboxEvent{}, err
	}

	return event, nil
}

func (r *PostgresOutboxRepository) ClaimNextPending(ctx context.Context, eventType string) (OutboxEvent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return OutboxEvent{}, err
	}
	defer tx.Rollback()

	const selectQuery = `
		SELECT id, event_type, aggregate_type, aggregate_id, payload, status, created_at, processed_at
		FROM outbox_events
		WHERE status = $1
		  AND event_type = $2
		ORDER BY created_at ASC, id ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	var event OutboxEvent
	err = tx.QueryRowContext(ctx, selectQuery, OutboxStatusPending, eventType).Scan(
		&event.ID,
		&event.EventType,
		&event.AggregateType,
		&event.AggregateID,
		&event.Payload,
		&event.Status,
		&event.CreatedAt,
		&event.ProcessedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OutboxEvent{}, ErrNoPendingOutboxEvents
		}
		return OutboxEvent{}, err
	}

	if err := r.UpdateStatus(ctx, tx, event.ID, OutboxStatusProcessing, nil); err != nil {
		return OutboxEvent{}, err
	}

	event.Status = OutboxStatusProcessing
	if err := tx.Commit(); err != nil {
		return OutboxEvent{}, err
	}

	return event, nil
}

func (r *PostgresOutboxRepository) UpdateStatus(ctx context.Context, exec DBTX, eventID int64, status string, processedAt *time.Time) error {
	const query = `
		UPDATE outbox_events
		SET status = $2,
		    processed_at = $3
		WHERE id = $1
	`

	var timestamp any
	if processedAt != nil {
		value := processedAt.UTC()
		timestamp = value
	}

	_, err := exec.ExecContext(ctx, query, eventID, status, timestamp)
	return err
}
