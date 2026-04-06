package catalog

import (
	"context"
	"database/sql"
	"strings"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Service struct {
	db   *sql.DB
	repo Repository
}

func NewService(db *sql.DB, repo Repository) *Service {
	return &Service{
		db:   db,
		repo: repo,
	}
}

func (s *Service) ListReady(ctx context.Context, limit int) ([]Track, error) {
	return s.repo.ListReady(ctx, s.db, normalizeLimit(limit))
}

func (s *Service) FindReadyByID(ctx context.Context, trackID int64) (Track, error) {
	return s.repo.FindReadyByID(ctx, s.db, trackID)
}

func (s *Service) SearchReady(ctx context.Context, query string, limit int) ([]Track, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []Track{}, nil
	}

	return s.repo.SearchReady(ctx, s.db, query, normalizeLimit(limit))
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageSize
	}
	if limit > MaxPageSize {
		return MaxPageSize
	}

	return limit
}
