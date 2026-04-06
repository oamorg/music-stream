package app

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"music-stream/internal/catalog"
	"music-stream/internal/media"
	"music-stream/internal/platform/config"
	"music-stream/internal/platform/logging"
	"music-stream/internal/platform/store"
)

func RunWorker() error {
	cfg := config.Load()
	logger := logging.New(cfg.AppEnv)

	logger.Printf("worker starting service=%s env=%s", cfg.ServiceName, cfg.AppEnv)

	db, err := store.OpenPostgres(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	worker := media.NewTranscodeWorker(
		db,
		logger,
		catalog.NewPostgresRepository(),
		media.NewPostgresTrackAssetRepository(),
		store.NewPostgresOutboxRepository(db),
		cfg.FFmpegBinary,
		cfg.LocalMediaRoot,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(cfg.WorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Printf("worker shutdown requested")
			return nil
		case <-ticker.C:
			if err := worker.ProcessOnce(ctx); err != nil && !errors.Is(err, store.ErrNoPendingOutboxEvents) {
				logger.Printf("worker process error=%v", err)
			}
		}
	}
}
