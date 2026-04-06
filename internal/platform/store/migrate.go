package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type MigrationRunner struct {
	db             *sql.DB
	migrationsPath string
}

type migrationFile struct {
	Version string
	Path    string
}

func NewMigrationRunner(db *sql.DB, migrationsPath string) MigrationRunner {
	return MigrationRunner{
		db:             db,
		migrationsPath: migrationsPath,
	}
}

func (r MigrationRunner) Up() (int, error) {
	if err := r.ensureTable(); err != nil {
		return 0, err
	}

	appliedVersions, err := r.appliedVersions()
	if err != nil {
		return 0, err
	}

	files, err := r.listMigrationFiles(".up.sql")
	if err != nil {
		return 0, err
	}

	appliedCount := 0
	for _, file := range files {
		if appliedVersions[file.Version] {
			continue
		}

		if err := r.applyUpMigration(file); err != nil {
			return appliedCount, err
		}

		appliedCount++
	}

	return appliedCount, nil
}

func (r MigrationRunner) Down() (string, error) {
	if err := r.ensureTable(); err != nil {
		return "", err
	}

	applied, err := r.latestAppliedVersion()
	if err != nil {
		return "", err
	}
	if applied == "" {
		return "", nil
	}

	filePath := filepath.Join(r.migrationsPath, applied+".down.sql")
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(string(content)); err != nil {
		return "", err
	}

	if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version = $1`, applied); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return applied, nil
}

func (r MigrationRunner) ensureTable() error {
	const query = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`

	_, err := r.db.Exec(query)
	return err
}

func (r MigrationRunner) appliedVersions() (map[string]bool, error) {
	rows, err := r.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions[version] = true
	}

	return versions, rows.Err()
}

func (r MigrationRunner) latestAppliedVersion() (string, error) {
	row := r.db.QueryRow(`
		SELECT version
		FROM schema_migrations
		ORDER BY version DESC
		LIMIT 1
	`)

	var version string
	if err := row.Scan(&version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	return version, nil
}

func (r MigrationRunner) listMigrationFiles(suffix string) ([]migrationFile, error) {
	pattern := filepath.Join(r.migrationsPath, "*"+suffix)
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)

	files := make([]migrationFile, 0, len(paths))
	for _, path := range paths {
		base := filepath.Base(path)
		version := strings.TrimSuffix(base, suffix)
		files = append(files, migrationFile{
			Version: version,
			Path:    path,
		})
	}

	return files, nil
}

func (r MigrationRunner) applyUpMigration(file migrationFile) error {
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("apply migration %s: %w", file.Version, err)
	}

	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
		file.Version,
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("record migration %s: %w", file.Version, err)
	}

	return tx.Commit()
}
