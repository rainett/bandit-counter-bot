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
					prize_values TEXT NOT NULL DEFAULT '[64]',
					win_amount INTEGER NOT NULL DEFAULT 64,
					allow_user_settings INTEGER NOT NULL DEFAULT 0,
					allow_user_reset INTEGER NOT NULL DEFAULT 0
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

func TestGetPermission_DefaultFalse(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	allowed, err := repo.GetPermission(100, "settings")
	if err != nil {
		t.Fatalf("GetPermission() error = %v", err)
	}
	if allowed {
		t.Error("expected default permission to be false")
	}
}

func TestTogglePermission(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	result, err := repo.TogglePermission(100, "settings")
	if err != nil {
		t.Fatalf("TogglePermission() error = %v", err)
	}
	if !result {
		t.Error("expected permission to be true after toggle on")
	}

	result, err = repo.TogglePermission(100, "settings")
	if err != nil {
		t.Fatalf("TogglePermission() error = %v", err)
	}
	if result {
		t.Error("expected permission to be false after toggle off")
	}
}

func TestTogglePermission_Independent(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	repo.TogglePermission(100, "settings")

	allowSettings, _ := repo.GetPermission(100, "settings")
	allowReset, _ := repo.GetPermission(100, "reset")

	if !allowSettings {
		t.Error("settings permission should be true")
	}
	if allowReset {
		t.Error("reset permission should still be false")
	}
}

func TestGetPermission_InvalidAction(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	allowed, err := repo.GetPermission(100, "invalid")
	if err != nil {
		t.Fatalf("GetPermission() error = %v", err)
	}
	if allowed {
		t.Error("invalid action should return false")
	}
}

func TestTogglePermission_ChatIsolation(t *testing.T) {
	db := setupSettingsDB(t)
	defer db.Close()
	repo := NewSettingsRepo(db)

	repo.TogglePermission(100, "settings")

	allow100, _ := repo.GetPermission(100, "settings")
	allow200, _ := repo.GetPermission(200, "settings")

	if !allow100 {
		t.Error("chat 100 should have settings allowed")
	}
	if allow200 {
		t.Error("chat 200 should not have settings allowed")
	}
}
