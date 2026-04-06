package auth

import (
	"context"
	"sync"
	"time"
)

type InMemoryUserRepository struct {
	mu       sync.RWMutex
	nextID   int64
	byID     map[int64]User
	byEmail  map[string]int64
}

func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		nextID:  1,
		byID:    make(map[int64]User),
		byEmail: make(map[string]int64),
	}
}

func (r *InMemoryUserRepository) Create(_ context.Context, params CreateUserParams) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byEmail[params.Email]; exists {
		return User{}, ErrEmailAlreadyExists
	}

	now := time.Now().UTC()
	user := User{
		ID:           r.nextID,
		Email:        params.Email,
		PasswordHash: params.PasswordHash,
		Status:       params.Status,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	r.byID[user.ID] = user
	r.byEmail[user.Email] = user.ID
	r.nextID++

	return user, nil
}

func (r *InMemoryUserRepository) FindByEmail(_ context.Context, email string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.byEmail[email]
	if !exists {
		return User{}, ErrNotFound
	}

	user, ok := r.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}

	return user, nil
}

func (r *InMemoryUserRepository) FindByID(_ context.Context, id int64) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.byID[id]
	if !exists {
		return User{}, ErrNotFound
	}

	return user, nil
}

type InMemoryRefreshTokenRepository struct {
	mu      sync.RWMutex
	nextID  int64
	byID    map[int64]RefreshToken
	byHash  map[string]int64
}

func NewInMemoryRefreshTokenRepository() *InMemoryRefreshTokenRepository {
	return &InMemoryRefreshTokenRepository{
		nextID: 1,
		byID:   make(map[int64]RefreshToken),
		byHash: make(map[string]int64),
	}
}

func (r *InMemoryRefreshTokenRepository) Create(_ context.Context, params CreateRefreshTokenParams) (RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	token := RefreshToken{
		ID:        r.nextID,
		UserID:    params.UserID,
		TokenHash: params.TokenHash,
		ExpiresAt: params.ExpiresAt.UTC(),
		CreatedAt: now,
	}

	r.byID[token.ID] = token
	r.byHash[token.TokenHash] = token.ID
	r.nextID++

	return token, nil
}

func (r *InMemoryRefreshTokenRepository) FindActiveByHash(_ context.Context, tokenHash string, now time.Time) (RefreshToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.byHash[tokenHash]
	if !exists {
		return RefreshToken{}, ErrNotFound
	}

	token, ok := r.byID[id]
	if !ok {
		return RefreshToken{}, ErrNotFound
	}

	if token.RevokedAt != nil || !token.ExpiresAt.After(now.UTC()) {
		return RefreshToken{}, ErrNotFound
	}

	return token, nil
}

func (r *InMemoryRefreshTokenRepository) RevokeByID(_ context.Context, id int64, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	token, exists := r.byID[id]
	if !exists {
		return ErrNotFound
	}

	token.RevokedAt = ptrTime(revokedAt.UTC())
	r.byID[id] = token

	return nil
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
