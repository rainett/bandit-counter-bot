package repository

import (
	"database/sql"
	"testing"
	"testing/fstest"

	_ "github.com/mattn/go-sqlite3"
)

func TestParseMigrationVersion(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantVer   int
		wantError bool
	}{
		{"simple", "001_init.sql", 1, false},
		{"larger version", "042_add_column.sql", 42, false},
		{"with path", "migrations/003_settings.sql", 3, false},
		{"no suffix", "001_init.txt", 0, true},
		{"zero version", "000_bad.sql", 0, true},
		{"negative version", "-1_bad.sql", 0, true},
		{"no version", "_bad.sql", 0, true},
		{"non-numeric", "abc_bad.sql", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, err := ParseMigrationVersion(tt.filename)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseMigrationVersion(%q) error = %v, wantError %v", tt.filename, err, tt.wantError)
				return
			}
			if ver != tt.wantVer {
				t.Errorf("ParseMigrationVersion(%q) = %d, want %d", tt.filename, ver, tt.wantVer)
			}
		})
	}
}

func TestMigrate(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	migrations := fstest.MapFS{
		"001_create_table.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`),
		},
		"002_add_column.sql": &fstest.MapFile{
			Data: []byte(`ALTER TABLE test_items ADD COLUMN value INTEGER NOT NULL DEFAULT 0;`),
		},
	}

	if err := Migrate(db, migrations); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Verify schema_version was updated
	var version int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Errorf("schema_version = %d, want 2", version)
	}

	// Verify table was created with both columns
	_, err = db.Exec(`INSERT INTO test_items (id, name, value) VALUES (1, 'test', 42)`)
	if err != nil {
		t.Errorf("insert failed: %v", err)
	}

	// Run again — should be a no-op
	if err := Migrate(db, migrations); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versionAfter int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&versionAfter); err != nil {
		t.Fatal(err)
	}
	if versionAfter != 2 {
		t.Errorf("schema_version after re-run = %d, want 2", versionAfter)
	}
}

func TestMigrateSkipsInvalidFiles(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	migrations := fstest.MapFS{
		"001_valid.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE t (id INTEGER PRIMARY KEY);`),
		},
		"bad_no_version.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE bad (id INTEGER PRIMARY KEY);`),
		},
	}

	if err := Migrate(db, migrations); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Valid migration should have run
	var version int
	db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version)
	if version != 1 {
		t.Errorf("schema_version = %d, want 1", version)
	}

	// Invalid migration should not have created table
	_, err = db.Exec(`INSERT INTO bad (id) VALUES (1)`)
	if err == nil {
		t.Error("expected error inserting into non-existent 'bad' table")
	}
}
