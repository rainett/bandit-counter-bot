package repository

import (
	"database/sql"
	"testing"
	"testing/fstest"

	_ "github.com/mattn/go-sqlite3"
)

func setupSettingsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	migrations := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{
			Data: []byte(`
				CREATE TABLE IF NOT EXISTS chat_settings(
					chat_id INTEGER PRIMARY KEY,
					prize_values TEXT NOT NULL DEFAULT '[64]'
				);
			`),
		},
	}

	if err := Migrate(db, migrations); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestGetPrizeValues_Default(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	values, err := repo.GetPrizeValues(100)
	if err != nil {
		t.Fatalf("GetPrizeValues() error = %v", err)
	}
	if len(values) != 1 || values[0] != 64 {
		t.Errorf("default values = %v, want [64]", values)
	}
}

func TestUpdateAndGetPrizeValues(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	err := repo.UpdatePrizeValues("[1,22,43,64]", 100)
	if err != nil {
		t.Fatalf("UpdatePrizeValues() error = %v", err)
	}

	values, err := repo.GetPrizeValues(100)
	if err != nil {
		t.Fatal(err)
	}
	expected := []int{1, 22, 43, 64}
	if len(values) != len(expected) {
		t.Fatalf("values = %v, want %v", values, expected)
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestUpdatePrizeValues_Overwrite(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	repo.UpdatePrizeValues("[1,22,43,64]", 100)
	repo.UpdatePrizeValues("[43]", 100)

	values, err := repo.GetPrizeValues(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != 43 {
		t.Errorf("values = %v, want [43]", values)
	}
}

func TestGetPrizeValues_InvalidJSON(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	// Insert invalid JSON directly
	db.Exec(`INSERT INTO chat_settings (chat_id, prize_values) VALUES (100, 'not-json')`)

	values, err := repo.GetPrizeValues(100)
	if err != nil {
		t.Fatalf("GetPrizeValues() error = %v", err)
	}
	// Should fall back to default
	if len(values) != 1 || values[0] != 64 {
		t.Errorf("values = %v, want [64] (default)", values)
	}
}

func TestGetPrizeValues_EmptyArray(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	db.Exec(`INSERT INTO chat_settings (chat_id, prize_values) VALUES (100, '[]')`)

	values, err := repo.GetPrizeValues(100)
	if err != nil {
		t.Fatal(err)
	}
	// Should fall back to default
	if len(values) != 1 || values[0] != 64 {
		t.Errorf("values = %v, want [64] (default)", values)
	}
}

func TestPrizeValues_ChatIsolation(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	repo.UpdatePrizeValues("[43]", 100)
	repo.UpdatePrizeValues("[1,22,43,64]", 200)

	v100, _ := repo.GetPrizeValues(100)
	v200, _ := repo.GetPrizeValues(200)

	if len(v100) != 1 || v100[0] != 43 {
		t.Errorf("chat 100 values = %v, want [43]", v100)
	}
	if len(v200) != 4 {
		t.Errorf("chat 200 values = %v, want [1,22,43,64]", v200)
	}
}
