//go:build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"

	"music-stream/internal/auth"
	"music-stream/internal/catalog"
	"music-stream/internal/history"
	"music-stream/internal/media"
	"music-stream/internal/platform/config"
	"music-stream/internal/platform/httpx"
	"music-stream/internal/platform/store"
	"music-stream/internal/playback"
)

type healthResponse struct {
	Status string          `json:"status"`
	Checks map[string]bool `json:"checks"`
}

type registerResponse struct {
	User auth.User `json:"user"`
}

type tracksResponse struct {
	Items []catalog.Track `json:"items"`
}

type historyResponse struct {
	Items []history.Item `json:"items"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestMVPFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ffmpegBinary := strings.TrimSpace(os.Getenv("FFMPEG_BINARY"))
	if ffmpegBinary == "" {
		t.Skip("set FFMPEG_BINARY to a real ffmpeg binary")
	}
	if _, err := os.Stat(ffmpegBinary); err != nil {
		t.Skipf("ffmpeg binary is not accessible: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	workDir := t.TempDir()
	mediaRoot := filepath.Join(workDir, "media")
	if err := os.MkdirAll(mediaRoot, 0o755); err != nil {
		t.Fatalf("create media root: %v", err)
	}

	pgPort := reserveTCPPort(t)
	postgres := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Port(uint32(pgPort)).
			Database("music").
			Username("music").
			Password("music").
			Version(embeddedpostgres.V17).
			RuntimePath(filepath.Join(workDir, "postgres-runtime")).
			DataPath(filepath.Join(workDir, "postgres-data")).
			BinariesPath(filepath.Join(workDir, "postgres-binaries")).
			StartTimeout(2 * time.Minute).
			BinaryRepositoryURL(postgresBinaryRepositoryURL()),
	)
	if err := postgres.Start(); err != nil {
		t.Fatalf("start embedded postgres: %v", err)
	}
	defer func() {
		if err := postgres.Stop(); err != nil {
			t.Fatalf("stop embedded postgres: %v", err)
		}
	}()

	redisAddr, stopRedis := startDummyTCPService(t)
	defer stopRedis()
	minioAddr, stopMinIO := startDummyTCPService(t)
	defer stopMinIO()

	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen http: %v", err)
	}
	httpAddr := httpListener.Addr().String()

	cfg := config.Config{
		ServiceName:         "music-stream",
		AppEnv:              "integration",
		HTTPPort:            portFromAddr(httpAddr),
		ShutdownTimeout:     5 * time.Second,
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     24 * time.Hour,
		ManifestURLTTL:      10 * time.Minute,
		AuthLoginRateLimit:  100,
		AuthLoginRateWindow: time.Minute,
		EventRateLimit:      1000,
		EventRateWindow:     time.Minute,
		WorkerPollInterval:  time.Second,
		DatabaseURL:         fmt.Sprintf("postgres://music:music@127.0.0.1:%d/music?sslmode=disable", pgPort),
		RedisAddr:           redisAddr,
		MinIOEndpoint:       minioAddr,
		MediaBaseURL:        "http://" + httpAddr + "/media",
		LocalMediaRoot:      mediaRoot,
		FFmpegBinary:        ffmpegBinary,
		JWTAccessSecret:     "integration-access-secret",
		JWTRefreshSecret:    "integration-refresh-secret",
	}

	db, err := store.OpenPostgres(cfg)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	root := projectRoot(t)
	migrations := store.NewMigrationRunner(db, filepath.Join(root, "db", "migrations"))
	if _, err := migrations.Up(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	userRepo := auth.NewPostgresUserRepository(db)
	tokenManager := auth.NewTokenManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)

	authService := auth.NewService(
		userRepo,
		auth.NewPostgresRefreshTokenRepository(db),
		auth.NewPasswordHasher(auth.DefaultPasswordIterations, auth.DefaultPasswordKeyLength, auth.DefaultPasswordSaltLength),
		tokenManager,
		time.Now,
	)
	authenticator := auth.NewAuthenticator(userRepo, tokenManager, time.Now)
	loginLimiter := httpx.NewFixedWindowRateLimiter(cfg.AuthLoginRateLimit, cfg.AuthLoginRateWindow, time.Now)
	eventLimiter := httpx.NewFixedWindowRateLimiter(cfg.EventRateLimit, cfg.EventRateWindow, time.Now)

	catalogService := catalog.NewService(db, catalog.NewPostgresRepository())
	assetRepo := media.NewPostgresTrackAssetRepository()
	mediaLookupService := media.NewLookupService(db, assetRepo)
	playbackService := playback.NewService(
		playback.NewPostgresRepository(db),
		catalogService,
		mediaLookupService,
		cfg.MediaBaseURL,
		cfg.ManifestURLTTL,
		time.Now,
	)
	historyService := history.NewService(history.NewPostgresRepository(db))
	importService := media.NewImportService(
		db,
		catalog.NewPostgresRepository(),
		assetRepo,
		store.NewPostgresOutboxRepository(db),
		time.Now,
	)
	worker := media.NewTranscodeWorker(
		db,
		log.New(io.Discard, "", 0),
		catalog.NewPostgresRepository(),
		assetRepo,
		store.NewPostgresOutboxRepository(db),
		cfg.FFmpegBinary,
		cfg.LocalMediaRoot,
	)

	router := httpx.NewRouter(log.New(io.Discard, "", 0), cfg, httpx.Dependencies{
		Auth: auth.NewHandler(authService, auth.HandlerOptions{
			LoginLimiter: loginLimiter,
		}),
		Catalog: catalog.NewHandler(catalogService),
		Playback: playback.NewHandler(playbackService, authenticator, playback.HandlerOptions{
			EventLimiter: eventLimiter,
		}),
		History:   history.NewHandler(historyService, authenticator),
		Readiness: store.NewReadinessChecker(db, cfg.RedisAddr, cfg.MinIOEndpoint),
	})

	server := &http.Server{Handler: router}
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Serve(httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)

		select {
		case err := <-serverErr:
			t.Fatalf("http server failed: %v", err)
		default:
		}
	}()

	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := "http://" + httpAddr

	live := requestJSON[map[string]any](t, client, http.MethodGet, baseURL+"/health/live", nil, "", http.StatusOK)
	if live["status"] != "ok" {
		t.Fatalf("/health/live status = %v, want ok", live["status"])
	}

	ready := requestJSON[healthResponse](t, client, http.MethodGet, baseURL+"/health/ready", nil, "", http.StatusOK)
	if ready.Status != "ok" {
		t.Fatalf("/health/ready status = %q, want ok", ready.Status)
	}
	if !ready.Checks["database"] || !ready.Checks["redis"] || !ready.Checks["minio"] {
		t.Fatalf("/health/ready checks = %+v, want all true", ready.Checks)
	}

	register := requestJSON[registerResponse](t, client, http.MethodPost, baseURL+"/api/v1/auth/register", auth.RegisterInput{
		Email:    "demo@example.com",
		Password: "super-secret-password",
	}, "", http.StatusCreated)

	login := requestJSON[auth.AuthResult](t, client, http.MethodPost, baseURL+"/api/v1/auth/login", auth.LoginInput{
		Email:    "demo@example.com",
		Password: "super-secret-password",
	}, "", http.StatusOK)

	sourceKey := filepath.ToSlash(filepath.Join("raw", "e2e.wav"))
	sourcePath := filepath.Join(mediaRoot, filepath.FromSlash(sourceKey))
	if err := writeSilentWAV(sourcePath, time.Second); err != nil {
		t.Fatalf("write test wav: %v", err)
	}

	imported, err := importService.ImportTrack(ctx, media.ImportTrackInput{
		Title:           "E2E Song",
		ArtistName:      "E2E Artist",
		DurationSec:     1,
		SourceObjectKey: sourceKey,
	})
	if err != nil {
		t.Fatalf("import track: %v", err)
	}

	if err := worker.ProcessOnce(ctx); err != nil {
		t.Fatalf("worker process once: %v", err)
	}

	track, err := catalogService.FindReadyByID(ctx, imported.Track.ID)
	if err != nil {
		t.Fatalf("find ready track: %v", err)
	}
	asset, err := mediaLookupService.FindReadyAssetByTrackID(ctx, imported.Track.ID)
	if err != nil {
		t.Fatalf("find ready asset: %v", err)
	}
	if asset.HLSManifestKey == nil || strings.TrimSpace(*asset.HLSManifestKey) == "" {
		t.Fatalf("asset manifest key is empty: %+v", asset)
	}

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO user_entitlements (user_id, track_id, access_type) VALUES ($1, $2, $3)`,
		register.User.ID,
		track.ID,
		playback.AccessTypeStream,
	); err != nil {
		t.Fatalf("insert entitlement: %v", err)
	}

	tracks := requestJSON[tracksResponse](t, client, http.MethodGet, baseURL+"/api/v1/tracks?limit=10", nil, "", http.StatusOK)
	if len(tracks.Items) == 0 {
		t.Fatalf("/api/v1/tracks returned no items")
	}

	search := requestJSON[tracksResponse](t, client, http.MethodGet, baseURL+"/api/v1/search?q=E2E", nil, "", http.StatusOK)
	if len(search.Items) != 1 || search.Items[0].ID != track.ID {
		t.Fatalf("/api/v1/search items = %+v, want track %d", search.Items, track.ID)
	}

	trackDetail := requestJSON[catalog.Track](t, client, http.MethodGet, fmt.Sprintf("%s/api/v1/tracks/%d", baseURL, track.ID), nil, "", http.StatusOK)
	if trackDetail.Status != catalog.TrackStatusReady {
		t.Fatalf("/api/v1/tracks/{id} status = %q, want %q", trackDetail.Status, catalog.TrackStatusReady)
	}

	session := requestJSON[playback.PlaybackSession](t, client, http.MethodPost, baseURL+"/api/v1/playback/sessions", playback.CreateSessionInput{
		TrackID: track.ID,
	}, login.Tokens.AccessToken, http.StatusOK)
	if !strings.Contains(session.ManifestURL, "/media/hls/asset-") {
		t.Fatalf("manifest url = %q, want media path", session.ManifestURL)
	}

	manifestBody := requestText(t, client, http.MethodGet, session.ManifestURL, nil, "", http.StatusOK)
	if !strings.Contains(manifestBody, "#EXTM3U") {
		t.Fatalf("manifest is not valid m3u8:\n%s", manifestBody)
	}

	segmentURL := resolveSegmentURL(t, session.ManifestURL, manifestBody)
	segmentBytes := requestBytes(t, client, http.MethodGet, segmentURL, nil, "", http.StatusOK)
	if len(segmentBytes) == 0 {
		t.Fatalf("segment download returned empty body")
	}

	startEvent := requestJSON[playback.PlayEvent](t, client, http.MethodPost, baseURL+"/api/v1/playback/events", playback.ReportEventInput{
		SessionID:       session.ID,
		EventType:       playback.EventTypeStart,
		PositionSec:     0,
		ClientTimestamp: time.Now().UTC(),
	}, login.Tokens.AccessToken, http.StatusAccepted)
	if startEvent.EventType != playback.EventTypeStart {
		t.Fatalf("start event type = %q, want %q", startEvent.EventType, playback.EventTypeStart)
	}

	requestJSON[playback.PlayEvent](t, client, http.MethodPost, baseURL+"/api/v1/playback/events", playback.ReportEventInput{
		SessionID:       session.ID,
		EventType:       playback.EventTypeHeartbeat,
		PositionSec:     1,
		ClientTimestamp: time.Now().UTC().Add(10 * time.Second),
	}, login.Tokens.AccessToken, http.StatusAccepted)

	completeEvent := requestJSON[playback.PlayEvent](t, client, http.MethodPost, baseURL+"/api/v1/playback/events", playback.ReportEventInput{
		SessionID:       session.ID,
		EventType:       playback.EventTypeComplete,
		PositionSec:     1,
		ClientTimestamp: time.Now().UTC().Add(20 * time.Second),
	}, login.Tokens.AccessToken, http.StatusAccepted)
	if completeEvent.EventType != playback.EventTypeComplete {
		t.Fatalf("complete event type = %q, want %q", completeEvent.EventType, playback.EventTypeComplete)
	}

	historyItems := requestJSON[historyResponse](t, client, http.MethodGet, baseURL+"/api/v1/me/history?limit=10", nil, login.Tokens.AccessToken, http.StatusOK)
	if len(historyItems.Items) != 1 {
		t.Fatalf("/api/v1/me/history items = %d, want 1", len(historyItems.Items))
	}
	if historyItems.Items[0].TrackID != track.ID {
		t.Fatalf("history track id = %d, want %d", historyItems.Items[0].TrackID, track.ID)
	}
	if historyItems.Items[0].LastEventType != playback.EventTypeComplete {
		t.Fatalf("history last event = %q, want %q", historyItems.Items[0].LastEventType, playback.EventTypeComplete)
	}

	refreshed := requestJSON[auth.AuthResult](t, client, http.MethodPost, baseURL+"/api/v1/auth/refresh", auth.RefreshInput{
		RefreshToken: login.Tokens.RefreshToken,
	}, "", http.StatusOK)
	if refreshed.User.ID != register.User.ID {
		t.Fatalf("refresh user id = %d, want %d", refreshed.User.ID, register.User.ID)
	}

	requestStatus(t, client, http.MethodPost, baseURL+"/api/v1/auth/logout", auth.LogoutInput{
		RefreshToken: refreshed.Tokens.RefreshToken,
	}, "", http.StatusNoContent)

	refreshAfterLogout := requestError(t, client, http.MethodPost, baseURL+"/api/v1/auth/refresh", auth.RefreshInput{
		RefreshToken: refreshed.Tokens.RefreshToken,
	}, "", http.StatusUnauthorized)
	if refreshAfterLogout.Error.Code != "invalid_refresh_token" {
		t.Fatalf("refresh after logout error code = %q, want invalid_refresh_token", refreshAfterLogout.Error.Code)
	}

	metrics := requestText(t, client, http.MethodGet, baseURL+"/metrics", nil, "", http.StatusOK)
	assertContains(t, metrics, `dependency_up{service="music-stream",dependency="database"} 1`)
	assertContains(t, metrics, `dependency_up{service="music-stream",dependency="redis"} 1`)
	assertContains(t, metrics, `dependency_up{service="music-stream",dependency="minio"} 1`)
	assertContains(t, metrics, `http_requests_total{service="music-stream",method="GET",path="/health/ready",status="200"} 1`)
	assertContains(t, metrics, `http_requests_total{service="music-stream",method="POST",path="/api/v1/playback/events",status="202"} 3`)
}

func projectRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func startDummyTCPService(t *testing.T) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start dummy tcp service: %v", err)
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			_ = conn.Close()
		}
	}()

	return listener.Addr().String(), func() {
		close(done)
		_ = listener.Close()
	}
}

func portFromAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}

	return port
}

func postgresBinaryRepositoryURL() string {
	if raw := strings.TrimSpace(os.Getenv("E2E_POSTGRES_BINARY_REPOSITORY_URL")); raw != "" {
		return raw
	}

	return "https://repo.maven.apache.org/maven2"
}

func requestJSON[T any](
	t *testing.T,
	client *http.Client,
	method string,
	rawURL string,
	body any,
	accessToken string,
	wantStatus int,
) T {
	t.Helper()

	response := requestRaw(t, client, method, rawURL, body, accessToken, wantStatus)
	defer response.Body.Close()

	var result T
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode json %s %s: %v", method, rawURL, err)
	}

	return result
}

func requestError(
	t *testing.T,
	client *http.Client,
	method string,
	rawURL string,
	body any,
	accessToken string,
	wantStatus int,
) errorResponse {
	t.Helper()

	return requestJSON[errorResponse](t, client, method, rawURL, body, accessToken, wantStatus)
}

func requestStatus(
	t *testing.T,
	client *http.Client,
	method string,
	rawURL string,
	body any,
	accessToken string,
	wantStatus int,
) {
	t.Helper()

	response := requestRaw(t, client, method, rawURL, body, accessToken, wantStatus)
	defer response.Body.Close()
}

func requestText(
	t *testing.T,
	client *http.Client,
	method string,
	rawURL string,
	body any,
	accessToken string,
	wantStatus int,
) string {
	t.Helper()

	response := requestRaw(t, client, method, rawURL, body, accessToken, wantStatus)
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body %s %s: %v", method, rawURL, err)
	}

	return string(payload)
}

func requestBytes(
	t *testing.T,
	client *http.Client,
	method string,
	rawURL string,
	body any,
	accessToken string,
	wantStatus int,
) []byte {
	t.Helper()

	response := requestRaw(t, client, method, rawURL, body, accessToken, wantStatus)
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response bytes %s %s: %v", method, rawURL, err)
	}

	return payload
}

func requestRaw(
	t *testing.T,
	client *http.Client,
	method string,
	rawURL string,
	body any,
	accessToken string,
	wantStatus int,
) *http.Response {
	t.Helper()

	var requestBody io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body %s %s: %v", method, rawURL, err)
		}
		requestBody = bytes.NewReader(payload)
	}

	request, err := http.NewRequest(method, rawURL, requestBody)
	if err != nil {
		t.Fatalf("create request %s %s: %v", method, rawURL, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute request %s %s: %v", method, rawURL, err)
	}
	if response.StatusCode != wantStatus {
		defer response.Body.Close()
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("%s %s status = %d, want %d, body=%s", method, rawURL, response.StatusCode, wantStatus, strings.TrimSpace(string(payload)))
	}

	return response
}

func writeSilentWAV(path string, duration time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	const (
		sampleRate    = 8000
		channelCount  = 1
		bitsPerSample = 16
	)

	sampleCount := int(duration.Seconds() * sampleRate)
	if sampleCount <= 0 {
		sampleCount = sampleRate
	}

	bytesPerSample := channelCount * bitsPerSample / 8
	dataSize := sampleCount * bytesPerSample

	var buffer bytes.Buffer
	buffer.Grow(44 + dataSize)

	buffer.WriteString("RIFF")
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(36+dataSize))
	buffer.WriteString("WAVE")
	buffer.WriteString("fmt ")
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(channelCount))
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate*bytesPerSample))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(bytesPerSample))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(bitsPerSample))
	buffer.WriteString("data")
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(dataSize))
	buffer.Write(make([]byte, dataSize))

	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

func resolveSegmentURL(t *testing.T, manifestURL, manifestBody string) string {
	t.Helper()

	base, err := url.Parse(manifestURL)
	if err != nil {
		t.Fatalf("parse manifest url: %v", err)
	}

	for _, line := range strings.Split(manifestBody, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		segment, err := url.Parse(line)
		if err != nil {
			t.Fatalf("parse segment line %q: %v", line, err)
		}
		return base.ResolveReference(segment).String()
	}

	t.Fatal("manifest did not contain a media segment")
	return ""
}

func assertContains(t *testing.T, body, want string) {
	t.Helper()

	if !strings.Contains(body, want) {
		t.Fatalf("body missing %q:\n%s", want, body)
	}
}
