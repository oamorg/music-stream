package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, params CreateUserParams) (User, error) {
	const query = `
		INSERT INTO users (email, password_hash, status)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, status, created_at, updated_at
	`

	var user User
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.Email,
		params.PasswordHash,
		params.Status,
	).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailAlreadyExists
		}
		return User{}, err
	}

	return user, nil
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	const query = `
		SELECT id, email, password_hash, status, created_at, updated_at
		FROM users
		WHERE email = $1
		LIMIT 1
	`

	var user User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	return user, nil
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id int64) (User, error) {
	const query = `
		SELECT id, email, password_hash, status, created_at, updated_at
		FROM users
		WHERE id = $1
		LIMIT 1
	`

	var user User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	return user, nil
}

type PostgresRefreshTokenRepository struct {
	db *sql.DB
}

func NewPostgresRefreshTokenRepository(db *sql.DB) *PostgresRefreshTokenRepository {
	return &PostgresRefreshTokenRepository{db: db}
}

func (r *PostgresRefreshTokenRepository) Create(ctx context.Context, params CreateRefreshTokenParams) (RefreshToken, error) {
	const query = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at
	`

	var token RefreshToken
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.UserID,
		params.TokenHash,
		params.ExpiresAt.UTC(),
	).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	if err != nil {
		return RefreshToken{}, err
	}

	return token, nil
}

func (r *PostgresRefreshTokenRepository) FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > $2
		LIMIT 1
	`

	var token RefreshToken
	err := r.db.QueryRowContext(ctx, query, tokenHash, now.UTC()).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RefreshToken{}, ErrNotFound
		}
		return RefreshToken{}, err
	}

	return token, nil
}

func (r *PostgresRefreshTokenRepository) RevokeByID(ctx context.Context, id int64, revokedAt time.Time) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE id = $1
		  AND revoked_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id, revokedAt.UTC())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23505"
	}

	return false
}
