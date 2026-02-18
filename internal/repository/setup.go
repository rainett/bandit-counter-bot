package repository

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strconv"
	"strings"
)

func Migrate(db *sql.DB, migrationsFS fs.FS) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		);
	`)
	if err != nil {
		return err
	}

	var current int
	err = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current)
	if err != nil {
		return err
	}

	entries, err := fs.Glob(migrationsFS, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)

	for _, name := range entries {
		version, err := ParseMigrationVersion(name)
		if err != nil {
			log.Printf("skipping invalid migration file %s: %v", name, err)
			continue
		}
		if version <= current {
			continue
		}

		sqlBytes, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", version, err)
		}

		log.Printf("migrating schema version %d", version)
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", version, err)
		}

		if _, err := tx.Exec(`INSERT OR REPLACE INTO schema_version(version) VALUES (?)`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
	}

	return nil
}

func ParseMigrationVersion(filename string) (int, error) {
	base := filename
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		base = filename[idx+1:]
	}

	if !strings.HasSuffix(base, ".sql") {
		return 0, fmt.Errorf("migration %q: invalid extension", base)
	}

	name := strings.TrimSuffix(base, ".sql")
	parts := strings.SplitN(name, "_", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("migration %q: missing version prefix", base)
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("migration %q: invalid version number", base)
	}

	return version, nil
}
