package history

import (
	"context"
	"testing"
)

type stubRepository struct {
	listRecentByUserFunc func(context.Context, int64, int) ([]Item, error)
}

func (s stubRepository) ListRecentByUser(ctx context.Context, userID int64, limit int) ([]Item, error) {
	if s.listRecentByUserFunc == nil {
		panic("unexpected ListRecentByUser call")
	}
	return s.listRecentByUserFunc(ctx, userID, limit)
}

func TestServiceListRecentByUserNormalizesLimit(t *testing.T) {
	tests := []struct {
		name      string
		input     int
		wantLimit int
	}{
		{name: "default", input: 0, wantLimit: DefaultHistoryLimit},
		{name: "max", input: MaxHistoryLimit + 10, wantLimit: MaxHistoryLimit},
		{name: "explicit", input: 5, wantLimit: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotLimit int
			service := NewService(stubRepository{
				listRecentByUserFunc: func(_ context.Context, _ int64, limit int) ([]Item, error) {
					gotLimit = limit
					return []Item{}, nil
				},
			})

			if _, err := service.ListRecentByUser(context.Background(), 42, tc.input); err != nil {
				t.Fatalf("ListRecentByUser() error = %v", err)
			}
			if gotLimit != tc.wantLimit {
				t.Fatalf("ListRecentByUser() limit = %d, want %d", gotLimit, tc.wantLimit)
			}
		})
	}
}
