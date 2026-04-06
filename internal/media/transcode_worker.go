package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"music-stream/internal/catalog"
	"music-stream/internal/platform/store"
)

const defaultAudioCodec = "aac"

type TranscodeWorker struct {
	db             *sql.DB
	logger         *log.Logger
	tracks         catalog.Repository
	assets         TrackAssetRepository
	outbox         *store.PostgresOutboxRepository
	ffmpegBinary   string
	localMediaRoot string
}

func NewTranscodeWorker(
	db *sql.DB,
	logger *log.Logger,
	tracks catalog.Repository,
	assets TrackAssetRepository,
	outbox *store.PostgresOutboxRepository,
	ffmpegBinary string,
	localMediaRoot string,
) *TranscodeWorker {
	return &TranscodeWorker{
		db:             db,
		logger:         logger,
		tracks:         tracks,
		assets:         assets,
		outbox:         outbox,
		ffmpegBinary:   ffmpegBinary,
		localMediaRoot: localMediaRoot,
	}
}

func (w *TranscodeWorker) ProcessOnce(ctx context.Context) error {
	event, err := w.outbox.ClaimNextPending(ctx, EventTypeTrackTranscodeRequested)
	if err != nil {
		if err == store.ErrNoPendingOutboxEvents {
			return nil
		}
		return err
	}

	var payload TranscodeRequestedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return w.failEvent(ctx, event.ID, payload.TrackID, payload.AssetID, fmt.Sprintf("decode outbox payload: %v", err))
	}

	manifestKey, err := w.transcode(ctx, payload)
	if err != nil {
		return w.failEvent(ctx, event.ID, payload.TrackID, payload.AssetID, err.Error())
	}

	return w.completeEvent(ctx, event.ID, payload.TrackID, payload.AssetID, manifestKey)
}

func (w *TranscodeWorker) transcode(ctx context.Context, payload TranscodeRequestedPayload) (string, error) {
	sourcePath := w.resolveSourcePath(payload.SourceObjectKey)
	outputDir := filepath.Join(w.localMediaRoot, "hls", fmt.Sprintf("asset-%d", payload.AssetID))
	manifestPath := filepath.Join(outputDir, "index.m3u8")

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	cmd := exec.CommandContext(
		ctx,
		w.ffmpegBinary,
		"-y",
		"-i", sourcePath,
		"-c:a", defaultAudioCodec,
		"-b:a", "128k",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.ts"),
		manifestPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg failed: %v: %s", err, strings.TrimSpace(string(output)))
	}

	manifestKey := filepath.ToSlash(filepath.Join("hls", fmt.Sprintf("asset-%d", payload.AssetID), "index.m3u8"))
	return manifestKey, nil
}

func (w *TranscodeWorker) completeEvent(ctx context.Context, eventID, trackID, assetID int64, manifestKey string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := w.assets.MarkReady(ctx, tx, assetID, manifestKey, defaultAudioCodec, 128); err != nil {
		return err
	}
	if err := w.tracks.UpdateStatus(ctx, tx, trackID, catalog.TrackStatusReady); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := w.outbox.UpdateStatus(ctx, tx, eventID, store.OutboxStatusProcessed, &now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	w.logger.Printf("worker transcode success event_id=%d track_id=%d asset_id=%d manifest_key=%s", eventID, trackID, assetID, manifestKey)
	return nil
}

func (w *TranscodeWorker) failEvent(ctx context.Context, eventID, trackID, assetID int64, message string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if assetID > 0 {
		if err := w.assets.MarkFailed(ctx, tx, assetID, message); err != nil {
			return err
		}
	}
	if trackID > 0 {
		if err := w.tracks.UpdateStatus(ctx, tx, trackID, catalog.TrackStatusBlocked); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	if err := w.outbox.UpdateStatus(ctx, tx, eventID, store.OutboxStatusFailed, &now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	w.logger.Printf("worker transcode failed event_id=%d track_id=%d asset_id=%d error=%q", eventID, trackID, assetID, message)
	return nil
}

func (w *TranscodeWorker) resolveSourcePath(sourceObjectKey string) string {
	sourceObjectKey = strings.TrimSpace(sourceObjectKey)
	if filepath.IsAbs(sourceObjectKey) {
		return sourceObjectKey
	}

	return filepath.Join(w.localMediaRoot, filepath.FromSlash(sourceObjectKey))
}
