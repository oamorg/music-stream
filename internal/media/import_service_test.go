package media

import (
	"testing"
	"time"
)

func TestNormalizeImportTrackInput(t *testing.T) {
	albumName := " Album "
	coverURL := " https://example.com/cover.jpg "
	releaseDate := time.Date(2024, time.January, 2, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))

	input, err := normalizeImportTrackInput(ImportTrackInput{
		Title:           " Song ",
		ArtistName:      " Artist ",
		AlbumName:       &albumName,
		DurationSec:     180,
		ReleaseDate:     &releaseDate,
		CoverURL:        &coverURL,
		SourceObjectKey: " tracks/song.mp3 ",
	})
	if err != nil {
		t.Fatalf("normalizeImportTrackInput() error = %v", err)
	}

	if input.Title != "Song" {
		t.Fatalf("Title = %q, want %q", input.Title, "Song")
	}
	if input.ArtistName != "Artist" {
		t.Fatalf("ArtistName = %q, want %q", input.ArtistName, "Artist")
	}
	if input.SourceObjectKey != "tracks/song.mp3" {
		t.Fatalf("SourceObjectKey = %q, want trimmed value", input.SourceObjectKey)
	}
	if input.AlbumName == nil || *input.AlbumName != "Album" {
		t.Fatalf("AlbumName = %#v, want trimmed value", input.AlbumName)
	}
	if input.CoverURL == nil || *input.CoverURL != "https://example.com/cover.jpg" {
		t.Fatalf("CoverURL = %#v, want trimmed value", input.CoverURL)
	}
	if input.ReleaseDate == nil || input.ReleaseDate.Location() != time.UTC {
		t.Fatalf("ReleaseDate = %#v, want UTC time", input.ReleaseDate)
	}
}

func TestNormalizeImportTrackInputRejectsMissingFields(t *testing.T) {
	_, err := normalizeImportTrackInput(ImportTrackInput{})
	if err != ErrInvalidTrackTitle {
		t.Fatalf("error = %v, want %v", err, ErrInvalidTrackTitle)
	}
}

func TestNormalizeImportTrackInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		input ImportTrackInput
		want  error
	}{
		{
			name: "missing artist",
			input: ImportTrackInput{
				Title:           "Song",
				SourceObjectKey: "raw/song.mp3",
			},
			want: ErrInvalidArtistName,
		},
		{
			name: "negative duration",
			input: ImportTrackInput{
				Title:           "Song",
				ArtistName:      "Artist",
				DurationSec:     -1,
				SourceObjectKey: "raw/song.mp3",
			},
			want: ErrInvalidDuration,
		},
		{
			name: "missing source key",
			input: ImportTrackInput{
				Title:      "Song",
				ArtistName: "Artist",
			},
			want: ErrInvalidSourceObjectKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeImportTrackInput(tc.input)
			if err != tc.want {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}
