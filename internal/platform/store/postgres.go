package store

import (
	"database/sql"

	_ "github.com/lib/pq"

	"music-stream/internal/platform/config"
)

func OpenPostgres(cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	return db, nil
}
