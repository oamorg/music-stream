package main

import (
	"fmt"
	"log"
	"os"

	"music-stream/internal/platform/config"
	"music-stream/internal/platform/store"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: go run ./cmd/migrate [up|down]")
	}

	cfg := config.Load()
	db, err := store.OpenPostgres(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	runner := store.NewMigrationRunner(db, "db/migrations")

	switch os.Args[1] {
	case "up":
		applied, err := runner.Up()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("applied migrations: %d\n", applied)
	case "down":
		version, err := runner.Down()
		if err != nil {
			log.Fatal(err)
		}
		if version == "" {
			fmt.Println("no migration rolled back")
			return
		}
		fmt.Printf("rolled back migration: %s\n", version)
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}
