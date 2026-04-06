package media

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranscodeWorkerTranscodeCreatesManifestAndSegments(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "raw", "song.mp3")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("fixture audio bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile() source error = %v", err)
	}

	ffmpegBinary := filepath.Join(root, "fake-ffmpeg.sh")
	writeExecutable(t, ffmpegBinary, `#!/bin/sh
set -eu
prev=""
input=""
segment_pattern=""
manifest=""
for arg in "$@"; do
  if [ "$prev" = "-i" ]; then
    input="$arg"
  fi
  if [ "$prev" = "-hls_segment_filename" ]; then
    segment_pattern="$arg"
  fi
  prev="$arg"
  manifest="$arg"
done
if [ ! -f "$input" ]; then
  echo "missing input: $input" >&2
  exit 1
fi
mkdir -p "$(dirname "$manifest")"
printf '#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:6.0,\nsegment_000.ts\n' > "$manifest"
segment_path=$(printf "$segment_pattern" 0)
mkdir -p "$(dirname "$segment_path")"
printf 'segment-data' > "$segment_path"
`)

	worker := NewTranscodeWorker(nil, log.New(io.Discard, "", 0), nil, nil, nil, ffmpegBinary, root)

	manifestKey, err := worker.transcode(context.Background(), TranscodeRequestedPayload{
		AssetID:         8,
		SourceObjectKey: "raw/song.mp3",
	})
	if err != nil {
		t.Fatalf("transcode() error = %v", err)
	}

	if manifestKey != "hls/asset-8/index.m3u8" {
		t.Fatalf("manifestKey = %q, want %q", manifestKey, "hls/asset-8/index.m3u8")
	}

	manifestPath := filepath.Join(root, "hls", "asset-8", "index.m3u8")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() manifest error = %v", err)
	}
	if !strings.Contains(string(manifestBytes), "#EXTM3U") {
		t.Fatalf("manifest contents = %q, want HLS header", string(manifestBytes))
	}

	segmentPath := filepath.Join(root, "hls", "asset-8", "segment_000.ts")
	if _, err := os.Stat(segmentPath); err != nil {
		t.Fatalf("Stat() segment error = %v", err)
	}
}

func TestTranscodeWorkerTranscodeIncludesCommandFailureOutput(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "raw", "song.mp3")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("fixture audio bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile() source error = %v", err)
	}

	ffmpegBinary := filepath.Join(root, "fake-ffmpeg-fail.sh")
	writeExecutable(t, ffmpegBinary, `#!/bin/sh
echo "synthetic ffmpeg failure" >&2
exit 1
`)

	worker := NewTranscodeWorker(nil, log.New(io.Discard, "", 0), nil, nil, nil, ffmpegBinary, root)

	_, err := worker.transcode(context.Background(), TranscodeRequestedPayload{
		AssetID:         8,
		SourceObjectKey: "raw/song.mp3",
	})
	if err == nil {
		t.Fatalf("transcode() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "synthetic ffmpeg failure") {
		t.Fatalf("error = %q, want command output", err.Error())
	}
}

func TestResolveSourcePath(t *testing.T) {
	root := t.TempDir()
	worker := NewTranscodeWorker(nil, log.New(io.Discard, "", 0), nil, nil, nil, "ffmpeg", root)

	if got := worker.resolveSourcePath("raw/song.mp3"); got != filepath.Join(root, "raw", "song.mp3") {
		t.Fatalf("relative path = %q", got)
	}

	absolute := filepath.Join(root, "absolute", "song.mp3")
	if got := worker.resolveSourcePath(absolute); got != absolute {
		t.Fatalf("absolute path = %q, want %q", got, absolute)
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
