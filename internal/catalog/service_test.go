package catalog

import (
	"context"
	"testing"

	"music-stream/internal/platform/store"
)

type stubRepository struct {
	listReadyFunc    func(context.Context, store.DBTX, int) ([]Track, error)
	findReadyByIDFunc func(context.Context, store.DBTX, int64) (Track, error)
	searchReadyFunc  func(context.Context, store.DBTX, string, int) ([]Track, error)
}

func (s stubRepository) Create(context.Context, store.DBTX, CreateTrackParams) (Track, error) {
	panic("unexpected Create call")
}

func (s stubRepository) UpdateStatus(context.Context, store.DBTX, int64, string) error {
	panic("unexpected UpdateStatus call")
}

func (s stubRepository) ListReady(ctx context.Context, exec store.DBTX, limit int) ([]Track, error) {
	if s.listReadyFunc == nil {
		panic("unexpected ListReady call")
	}
	return s.listReadyFunc(ctx, exec, limit)
}

func (s stubRepository) FindReadyByID(ctx context.Context, exec store.DBTX, trackID int64) (Track, error) {
	if s.findReadyByIDFunc == nil {
		panic("unexpected FindReadyByID call")
	}
	return s.findReadyByIDFunc(ctx, exec, trackID)
}

func (s stubRepository) SearchReady(ctx context.Context, exec store.DBTX, query string, limit int) ([]Track, error) {
	if s.searchReadyFunc == nil {
		panic("unexpected SearchReady call")
	}
	return s.searchReadyFunc(ctx, exec, query, limit)
}

func TestServiceListReadyNormalizesLimit(t *testing.T) {
	var gotLimit int
	service := NewService(nil, stubRepository{
		listReadyFunc: func(_ context.Context, _ store.DBTX, limit int) ([]Track, error) {
			gotLimit = limit
			return []Track{}, nil
		},
	})

	if _, err := service.ListReady(context.Background(), MaxPageSize+50); err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	if gotLimit != MaxPageSize {
		t.Fatalf("ListReady() limit = %d, want %d", gotLimit, MaxPageSize)
	}
}

func TestServiceSearchReadySkipsEmptyQuery(t *testing.T) {
	called := false
	service := NewService(nil, stubRepository{
		searchReadyFunc: func(_ context.Context, _ store.DBTX, _ string, _ int) ([]Track, error) {
			called = true
			return nil, nil
		},
	})

	items, err := service.SearchReady(context.Background(), "   ", 20)
	if err != nil {
		t.Fatalf("SearchReady() error = %v", err)
	}
	if called {
		t.Fatalf("SearchReady() unexpectedly called repository for empty query")
	}
	if len(items) != 0 {
		t.Fatalf("SearchReady() len = %d, want 0", len(items))
	}
}
