package main

import (
	"log"

	"music-stream/internal/platform/app"
)

func main() {
	if err := app.RunWorker(); err != nil {
		log.Fatal(err)
	}
}
