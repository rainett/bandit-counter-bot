package repository

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func Migrate(db *sql.DB) error {
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

	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		version, _ := ParseMigrationVersion(f)
		if version <= current {
			continue
		}

		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			return err
		}

		log.Printf("migrating schema version %d", version)
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return err
		}
	}

	return nil
}

func ParseMigrationVersion(filename string) (int, error) {
	base := filepath.Base(filename)

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
