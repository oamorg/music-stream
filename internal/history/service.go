package history

import "context"

const (
	DefaultHistoryLimit = 20
	MaxHistoryLimit     = 100
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListRecentByUser(ctx context.Context, userID int64, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = DefaultHistoryLimit
	}
	if limit > MaxHistoryLimit {
		limit = MaxHistoryLimit
	}

	return s.repo.ListRecentByUser(ctx, userID, limit)
}
