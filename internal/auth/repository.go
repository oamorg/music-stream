package auth

import (
	"context"
	"time"
)

type CreateUserParams struct {
	Email        string
	PasswordHash string
	Status       string
}

type CreateRefreshTokenParams struct {
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
}

type UserRepository interface {
	Create(ctx context.Context, params CreateUserParams) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id int64) (User, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, params CreateRefreshTokenParams) (RefreshToken, error)
	FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (RefreshToken, error)
	RevokeByID(ctx context.Context, id int64, revokedAt time.Time) error
}
