package playback

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"music-stream/internal/auth"
	"music-stream/internal/catalog"
	"music-stream/internal/media"
)

var (
	ErrForbidden           = errors.New("forbidden")
	ErrInvalidTrackID      = errors.New("invalid track id")
	ErrInvalidSessionID    = errors.New("invalid session id")
	ErrInvalidEventType    = errors.New("invalid event type")
	ErrInvalidPosition     = errors.New("invalid position")
	ErrInvalidClientTime   = errors.New("invalid client timestamp")
	ErrManifestUnavailable = errors.New("manifest unavailable")
)

type CatalogReader interface {
	FindReadyByID(ctx context.Context, trackID int64) (catalog.Track, error)
}

type MediaReader interface {
	FindReadyAssetByTrackID(ctx context.Context, trackID int64) (media.TrackAsset, error)
}

type Service struct {
	repo         Repository
	catalog      CatalogReader
	media        MediaReader
	mediaBaseURL string
	manifestTTL  time.Duration
	now          func() time.Time
}

func NewService(
	repo Repository,
	catalog CatalogReader,
	mediaReader MediaReader,
	mediaBaseURL string,
	manifestTTL time.Duration,
	now func() time.Time,
) *Service {
	if now == nil {
		now = time.Now
	}

	return &Service{
		repo:         repo,
		catalog:      catalog,
		media:        mediaReader,
		mediaBaseURL: strings.TrimRight(mediaBaseURL, "/"),
		manifestTTL:  manifestTTL,
		now:          now,
	}
}

func (s *Service) CreateSession(ctx context.Context, user auth.User, input CreateSessionInput) (PlaybackSession, error) {
	if input.TrackID <= 0 {
		return PlaybackSession{}, ErrInvalidTrackID
	}

	track, err := s.catalog.FindReadyByID(ctx, input.TrackID)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			return PlaybackSession{}, catalog.ErrNotFound
		}
		return PlaybackSession{}, err
	}

	if _, err := s.repo.FindActiveEntitlement(ctx, user.ID, track.ID, s.now().UTC()); err != nil {
		if errors.Is(err, ErrNotFound) {
			return PlaybackSession{}, ErrForbidden
		}
		return PlaybackSession{}, err
	}

	asset, err := s.media.FindReadyAssetByTrackID(ctx, track.ID)
	if err != nil {
		if errors.Is(err, media.ErrNotFound) {
			return PlaybackSession{}, ErrManifestUnavailable
		}
		return PlaybackSession{}, err
	}

	if asset.HLSManifestKey == nil || strings.TrimSpace(*asset.HLSManifestKey) == "" {
		return PlaybackSession{}, ErrManifestUnavailable
	}

	expiresAt := s.now().UTC().Add(s.manifestTTL)
	manifestURL := buildManifestURL(s.mediaBaseURL, *asset.HLSManifestKey, expiresAt)

	return s.repo.CreateSession(ctx, user.ID, track.ID, asset.ID, manifestURL, expiresAt)
}

func (s *Service) ReportEvent(ctx context.Context, user auth.User, input ReportEventInput) (PlayEvent, error) {
	if input.SessionID <= 0 {
		return PlayEvent{}, ErrInvalidSessionID
	}
	if input.PositionSec < 0 {
		return PlayEvent{}, ErrInvalidPosition
	}
	if input.ClientTimestamp.IsZero() {
		return PlayEvent{}, ErrInvalidClientTime
	}

	switch input.EventType {
	case EventTypeStart, EventTypeHeartbeat, EventTypeComplete:
	default:
		return PlayEvent{}, ErrInvalidEventType
	}

	session, err := s.repo.FindSessionByIDAndUser(ctx, input.SessionID, user.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return PlayEvent{}, ErrForbidden
		}
		return PlayEvent{}, err
	}

	return s.repo.CreateEvent(ctx, input, user.ID, session.TrackID)
}

func buildManifestURL(baseURL, manifestKey string, expiresAt time.Time) string {
	escapedKey := (&url.URL{Path: manifestKey}).EscapedPath()
	return fmt.Sprintf(
		"%s/%s?expires=%s",
		strings.TrimRight(baseURL, "/"),
		strings.TrimLeft(escapedKey, "/"),
		url.QueryEscape(strconv.FormatInt(expiresAt.Unix(), 10)),
	)
}
