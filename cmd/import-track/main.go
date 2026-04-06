package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"music-stream/internal/catalog"
	"music-stream/internal/media"
	"music-stream/internal/platform/config"
	"music-stream/internal/platform/store"
)

func main() {
	var (
		title           = flag.String("title", "", "track title")
		artistName      = flag.String("artist", "", "artist name")
		albumName       = flag.String("album", "", "album name")
		durationSec     = flag.Int("duration-sec", 0, "duration in seconds")
		releaseDate     = flag.String("release-date", "", "release date in YYYY-MM-DD")
		coverURL        = flag.String("cover-url", "", "optional cover URL")
		sourceObjectKey = flag.String("source-object-key", "", "uploaded source object key")
	)

	flag.Parse()

	cfg := config.Load()
	db, err := store.OpenPostgres(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	input, err := buildImportInput(
		*title,
		*artistName,
		*albumName,
		*durationSec,
		*releaseDate,
		*coverURL,
		*sourceObjectKey,
	)
	if err != nil {
		log.Fatal(err)
	}

	service := media.NewImportService(
		db,
		catalog.NewPostgresRepository(),
		media.NewPostgresTrackAssetRepository(),
		store.NewPostgresOutboxRepository(db),
		time.Now,
	)

	result, err := service.ImportTrack(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("imported track=%+v\n", result.Track)
	fmt.Printf("created asset=%+v\n", result.Asset)
	fmt.Printf("queued outbox=%+v\n", result.Outbox)
}

func buildImportInput(
	title string,
	artistName string,
	albumName string,
	durationSec int,
	releaseDate string,
	coverURL string,
	sourceObjectKey string,
) (media.ImportTrackInput, error) {
	input := media.ImportTrackInput{
		Title:           title,
		ArtistName:      artistName,
		DurationSec:     durationSec,
		SourceObjectKey: sourceObjectKey,
	}

	if albumName != "" {
		input.AlbumName = &albumName
	}
	if coverURL != "" {
		input.CoverURL = &coverURL
	}
	if releaseDate != "" {
		parsed, err := time.Parse("2006-01-02", releaseDate)
		if err != nil {
			return media.ImportTrackInput{}, fmt.Errorf("invalid release-date: %w", err)
		}
		input.ReleaseDate = &parsed
	}

	return input, nil
}
