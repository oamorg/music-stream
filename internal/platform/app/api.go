package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"music-stream/internal/auth"
	"music-stream/internal/catalog"
	"music-stream/internal/history"
	"music-stream/internal/media"
	"music-stream/internal/platform/config"
	"music-stream/internal/platform/httpx"
	"music-stream/internal/platform/logging"
	"music-stream/internal/platform/store"
	"music-stream/internal/playback"
)

func RunAPI() error {
	cfg := config.Load()
	logger := logging.New(cfg.AppEnv)

	db, err := store.OpenPostgres(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

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
	mediaLookupService := media.NewLookupService(db, media.NewPostgresTrackAssetRepository())
	playbackService := playback.NewService(
		playback.NewPostgresRepository(db),
		catalogService,
		mediaLookupService,
		cfg.MediaBaseURL,
		cfg.ManifestURLTTL,
		time.Now,
	)
	historyService := history.NewService(history.NewPostgresRepository(db))

	handler := httpx.NewRouter(logger, cfg, httpx.Dependencies{
		Auth: auth.NewHandler(authService, auth.HandlerOptions{
			LoginLimiter: loginLimiter,
		}),
		Catalog:   catalog.NewHandler(catalogService),
		Playback: playback.NewHandler(playbackService, authenticator, playback.HandlerOptions{
			EventLimiter: eventLimiter,
		}),
		History:   history.NewHandler(historyService, authenticator),
		Readiness: store.NewReadinessChecker(db, cfg.RedisAddr, cfg.MinIOEndpoint),
	})
	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("api starting service=%s env=%s addr=%s", cfg.ServiceName, cfg.AppEnv, server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Printf("api shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
