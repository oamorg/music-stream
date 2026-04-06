package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"music-stream/internal/auth"
	"music-stream/internal/catalog"
	"music-stream/internal/media"
)

type stubRepository struct {
	findActiveEntitlementFunc func(context.Context, int64, int64, time.Time) (UserEntitlement, error)
	createSessionFunc         func(context.Context, int64, int64, int64, string, time.Time) (PlaybackSession, error)
	findSessionByIDAndUserFunc func(context.Context, int64, int64) (PlaybackSession, error)
	createEventFunc           func(context.Context, ReportEventInput, int64, int64) (PlayEvent, error)
}

func (s stubRepository) FindActiveEntitlement(ctx context.Context, userID, trackID int64, now time.Time) (UserEntitlement, error) {
	if s.findActiveEntitlementFunc == nil {
		panic("unexpected FindActiveEntitlement call")
	}
	return s.findActiveEntitlementFunc(ctx, userID, trackID, now)
}

func (s stubRepository) CreateSession(ctx context.Context, userID, trackID, assetID int64, manifestURL string, expiresAt time.Time) (PlaybackSession, error) {
	if s.createSessionFunc == nil {
		panic("unexpected CreateSession call")
	}
	return s.createSessionFunc(ctx, userID, trackID, assetID, manifestURL, expiresAt)
}

func (s stubRepository) FindSessionByIDAndUser(ctx context.Context, sessionID, userID int64) (PlaybackSession, error) {
	if s.findSessionByIDAndUserFunc == nil {
		panic("unexpected FindSessionByIDAndUser call")
	}
	return s.findSessionByIDAndUserFunc(ctx, sessionID, userID)
}

func (s stubRepository) CreateEvent(ctx context.Context, input ReportEventInput, userID, trackID int64) (PlayEvent, error) {
	if s.createEventFunc == nil {
		panic("unexpected CreateEvent call")
	}
	return s.createEventFunc(ctx, input, userID, trackID)
}

type stubCatalogReader struct {
	findReadyByIDFunc func(context.Context, int64) (catalog.Track, error)
}

func (s stubCatalogReader) FindReadyByID(ctx context.Context, trackID int64) (catalog.Track, error) {
	if s.findReadyByIDFunc == nil {
		panic("unexpected FindReadyByID call")
	}
	return s.findReadyByIDFunc(ctx, trackID)
}

type stubMediaReader struct {
	findReadyAssetByTrackIDFunc func(context.Context, int64) (media.TrackAsset, error)
}

func (s stubMediaReader) FindReadyAssetByTrackID(ctx context.Context, trackID int64) (media.TrackAsset, error) {
	if s.findReadyAssetByTrackIDFunc == nil {
		panic("unexpected FindReadyAssetByTrackID call")
	}
	return s.findReadyAssetByTrackIDFunc(ctx, trackID)
}

func TestServiceCreateSessionBuildsManifestURL(t *testing.T) {
	now := time.Unix(1712360000, 0).UTC()
	manifestKey := "hls/asset-8/index.m3u8"

	var (
		gotManifestURL string
		gotExpiresAt   time.Time
	)

	service := NewService(
		stubRepository{
			findActiveEntitlementFunc: func(_ context.Context, userID, trackID int64, current time.Time) (UserEntitlement, error) {
				if userID != 7 || trackID != 9 {
					t.Fatalf("FindActiveEntitlement() got userID=%d trackID=%d", userID, trackID)
				}
				if !current.Equal(now) {
					t.Fatalf("FindActiveEntitlement() now = %v, want %v", current, now)
				}
				return UserEntitlement{ID: 1, UserID: userID, TrackID: trackID, AccessType: AccessTypeStream}, nil
			},
			createSessionFunc: func(_ context.Context, userID, trackID, assetID int64, manifestURL string, expiresAt time.Time) (PlaybackSession, error) {
				gotManifestURL = manifestURL
				gotExpiresAt = expiresAt
				return PlaybackSession{
					ID:          11,
					UserID:      userID,
					TrackID:     trackID,
					AssetID:     assetID,
					ManifestURL: manifestURL,
					ExpiresAt:   expiresAt,
					CreatedAt:   now,
				}, nil
			},
		},
		stubCatalogReader{
			findReadyByIDFunc: func(_ context.Context, trackID int64) (catalog.Track, error) {
				return catalog.Track{ID: trackID, Status: catalog.TrackStatusReady}, nil
			},
		},
		stubMediaReader{
			findReadyAssetByTrackIDFunc: func(_ context.Context, trackID int64) (media.TrackAsset, error) {
				return media.TrackAsset{ID: 8, TrackID: trackID, Status: media.TrackAssetStatusReady, HLSManifestKey: &manifestKey}, nil
			},
		},
		"http://localhost:8080/media/",
		10*time.Minute,
		func() time.Time { return now },
	)

	session, err := service.CreateSession(context.Background(), auth.User{ID: 7}, CreateSessionInput{TrackID: 9})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	wantManifestURL := "http://localhost:8080/media/hls/asset-8/index.m3u8?expires=1712360600"
	if gotManifestURL != wantManifestURL {
		t.Fatalf("CreateSession() manifestURL = %q, want %q", gotManifestURL, wantManifestURL)
	}
	if !gotExpiresAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("CreateSession() expiresAt = %v, want %v", gotExpiresAt, now.Add(10*time.Minute))
	}
	if session.ManifestURL != wantManifestURL {
		t.Fatalf("session.ManifestURL = %q, want %q", session.ManifestURL, wantManifestURL)
	}
}

func TestServiceCreateSessionRejectsMissingEntitlement(t *testing.T) {
	service := NewService(
		stubRepository{
			findActiveEntitlementFunc: func(context.Context, int64, int64, time.Time) (UserEntitlement, error) {
				return UserEntitlement{}, ErrNotFound
			},
		},
		stubCatalogReader{
			findReadyByIDFunc: func(_ context.Context, trackID int64) (catalog.Track, error) {
				return catalog.Track{ID: trackID, Status: catalog.TrackStatusReady}, nil
			},
		},
		stubMediaReader{},
		"http://localhost:8080/media",
		10*time.Minute,
		time.Now,
	)

	_, err := service.CreateSession(context.Background(), auth.User{ID: 7}, CreateSessionInput{TrackID: 9})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateSession() error = %v, want %v", err, ErrForbidden)
	}
}

func TestServiceCreateSessionRejectsInvalidTrackID(t *testing.T) {
	service := NewService(
		stubRepository{},
		stubCatalogReader{},
		stubMediaReader{},
		"http://localhost:8080/media",
		10*time.Minute,
		time.Now,
	)

	_, err := service.CreateSession(context.Background(), auth.User{ID: 7}, CreateSessionInput{})
	if !errors.Is(err, ErrInvalidTrackID) {
		t.Fatalf("CreateSession() error = %v, want %v", err, ErrInvalidTrackID)
	}
}

func TestServiceCreateSessionRejectsMissingManifest(t *testing.T) {
	service := NewService(
		stubRepository{
			findActiveEntitlementFunc: func(context.Context, int64, int64, time.Time) (UserEntitlement, error) {
				return UserEntitlement{ID: 1, UserID: 7, TrackID: 9, AccessType: AccessTypeStream}, nil
			},
		},
		stubCatalogReader{
			findReadyByIDFunc: func(_ context.Context, trackID int64) (catalog.Track, error) {
				return catalog.Track{ID: trackID, Status: catalog.TrackStatusReady}, nil
			},
		},
		stubMediaReader{
			findReadyAssetByTrackIDFunc: func(_ context.Context, trackID int64) (media.TrackAsset, error) {
				return media.TrackAsset{ID: 8, TrackID: trackID, Status: media.TrackAssetStatusReady}, nil
			},
		},
		"http://localhost:8080/media",
		10*time.Minute,
		time.Now,
	)

	_, err := service.CreateSession(context.Background(), auth.User{ID: 7}, CreateSessionInput{TrackID: 9})
	if !errors.Is(err, ErrManifestUnavailable) {
		t.Fatalf("CreateSession() error = %v, want %v", err, ErrManifestUnavailable)
	}
}

func TestServiceReportEventStoresValidatedEvent(t *testing.T) {
	now := time.Unix(1712360000, 0).UTC()
	wantInput := ReportEventInput{
		SessionID:       12,
		EventType:       EventTypeHeartbeat,
		PositionSec:     33,
		ClientTimestamp: now,
	}

	var gotTrackID int64

	service := NewService(
		stubRepository{
			findSessionByIDAndUserFunc: func(_ context.Context, sessionID, userID int64) (PlaybackSession, error) {
				if sessionID != 12 || userID != 5 {
					t.Fatalf("FindSessionByIDAndUser() got sessionID=%d userID=%d", sessionID, userID)
				}
				return PlaybackSession{ID: sessionID, UserID: userID, TrackID: 99}, nil
			},
			createEventFunc: func(_ context.Context, input ReportEventInput, userID, trackID int64) (PlayEvent, error) {
				if input != wantInput {
					t.Fatalf("CreateEvent() input = %#v, want %#v", input, wantInput)
				}
				if userID != 5 {
					t.Fatalf("CreateEvent() userID = %d, want 5", userID)
				}
				gotTrackID = trackID
				return PlayEvent{
					ID:              1,
					SessionID:       input.SessionID,
					UserID:          userID,
					TrackID:         trackID,
					EventType:       input.EventType,
					PositionSec:     input.PositionSec,
					ClientTimestamp: input.ClientTimestamp,
				}, nil
			},
		},
		stubCatalogReader{},
		stubMediaReader{},
		"http://localhost:8080/media",
		10*time.Minute,
		time.Now,
	)

	event, err := service.ReportEvent(context.Background(), auth.User{ID: 5}, wantInput)
	if err != nil {
		t.Fatalf("ReportEvent() error = %v", err)
	}
	if gotTrackID != 99 {
		t.Fatalf("CreateEvent() trackID = %d, want 99", gotTrackID)
	}
	if event.TrackID != 99 {
		t.Fatalf("event.TrackID = %d, want 99", event.TrackID)
	}
}

func TestServiceReportEventRejectsUnknownSession(t *testing.T) {
	service := NewService(
		stubRepository{
			findSessionByIDAndUserFunc: func(context.Context, int64, int64) (PlaybackSession, error) {
				return PlaybackSession{}, ErrNotFound
			},
		},
		stubCatalogReader{},
		stubMediaReader{},
		"http://localhost:8080/media",
		10*time.Minute,
		time.Now,
	)

	_, err := service.ReportEvent(context.Background(), auth.User{ID: 5}, ReportEventInput{
		SessionID:       12,
		EventType:       EventTypeStart,
		PositionSec:     0,
		ClientTimestamp: time.Now().UTC(),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ReportEvent() error = %v, want %v", err, ErrForbidden)
	}
}

func TestServiceReportEventRejectsInvalidPayload(t *testing.T) {
	service := NewService(
		stubRepository{},
		stubCatalogReader{},
		stubMediaReader{},
		"http://localhost:8080/media",
		10*time.Minute,
		time.Now,
	)

	tests := []struct {
		name  string
		input ReportEventInput
		want  error
	}{
		{
			name: "missing session",
			input: ReportEventInput{
				EventType:       EventTypeStart,
				PositionSec:     0,
				ClientTimestamp: time.Now().UTC(),
			},
			want: ErrInvalidSessionID,
		},
		{
			name: "negative position",
			input: ReportEventInput{
				SessionID:       1,
				EventType:       EventTypeStart,
				PositionSec:     -1,
				ClientTimestamp: time.Now().UTC(),
			},
			want: ErrInvalidPosition,
		},
		{
			name: "missing client time",
			input: ReportEventInput{
				SessionID:   1,
				EventType:   EventTypeStart,
				PositionSec: 0,
			},
			want: ErrInvalidClientTime,
		},
		{
			name: "invalid event type",
			input: ReportEventInput{
				SessionID:       1,
				EventType:       "PAUSE",
				PositionSec:     0,
				ClientTimestamp: time.Now().UTC(),
			},
			want: ErrInvalidEventType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.ReportEvent(context.Background(), auth.User{ID: 5}, tc.input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ReportEvent() error = %v, want %v", err, tc.want)
			}
		})
	}
}
