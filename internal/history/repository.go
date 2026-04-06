package history

import "context"

type Repository interface {
	ListRecentByUser(ctx context.Context, userID int64, limit int) ([]Item, error)
}
