package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"music-stream/internal/platform/config"
)

func main() {
	var (
		sourcePath = flag.String("source", "", "local source file path")
		targetKey  = flag.String("key", "", "target object key under local media root")
	)

	flag.Parse()

	if *sourcePath == "" {
		log.Fatal("source is required")
	}

	cfg := config.Load()
	key, err := stageFile(cfg.LocalMediaRoot, *sourcePath, *targetKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(key)
}

func stageFile(root, sourcePath, key string) (string, error) {
	if key == "" {
		key = filepath.ToSlash(filepath.Join("raw", filepath.Base(sourcePath)))
	}

	targetPath := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return "", err
	}

	return key, nil
}
