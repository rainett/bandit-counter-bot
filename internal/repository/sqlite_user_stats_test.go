package repository

import (
	"database/sql"
	"testing"
	"testing/fstest"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	migrations := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{
			Data: []byte(`
				CREATE TABLE IF NOT EXISTS user_stats (
					chat_id INTEGER NOT NULL,
					user_id INTEGER NOT NULL,
					username TEXT NOT NULL DEFAULT 'noname',
					spins INTEGER NOT NULL DEFAULT 0,
					wins INTEGER NOT NULL DEFAULT 0,
					balance INTEGER NOT NULL DEFAULT 0,
					current_streak INTEGER NOT NULL DEFAULT 0,
					max_streak INTEGER NOT NULL DEFAULT 0,
					current_loss_streak INTEGER NOT NULL DEFAULT 0,
					max_loss_streak INTEGER NOT NULL DEFAULT 0,
					PRIMARY KEY (chat_id, user_id)
				);
				CREATE INDEX IF NOT EXISTS user_stats_chat_balance_idx
				ON user_stats(chat_id, balance DESC);
			`),
		},
	}

	if err := Migrate(db, migrations); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSpin_NewUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	err := repo.Spin(100, 1, "alice", false, 64)
	if err != nil {
		t.Fatalf("Spin() error = %v", err)
	}

	stats, err := repo.GetPersonalStats(100, 1)
	if err != nil {
		t.Fatalf("GetPersonalStats() error = %v", err)
	}
	if stats.Spins != 1 {
		t.Errorf("Spins = %d, want 1", stats.Spins)
	}
	if stats.Wins != 0 {
		t.Errorf("Wins = %d, want 0", stats.Wins)
	}
	if stats.Balance != -1 {
		t.Errorf("Balance = %d, want -1", stats.Balance)
	}
}

func TestSpin_Win(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	err := repo.Spin(100, 1, "alice", true, 64)
	if err != nil {
		t.Fatalf("Spin() error = %v", err)
	}

	stats, err := repo.GetPersonalStats(100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Wins != 1 {
		t.Errorf("Wins = %d, want 1", stats.Wins)
	}
	if stats.Balance != 64 {
		t.Errorf("Balance = %d, want 64", stats.Balance)
	}
}

func TestSpin_UpdatesUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "old_name", false, 64)
	repo.Spin(100, 1, "new_name", false, 64)

	stats, err := repo.GetRichStats(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 user, got %d", len(stats))
	}
	if stats[0].Username != "new_name" {
		t.Errorf("Username = %q, want %q", stats[0].Username, "new_name")
	}
}

func TestSpin_AccumulatesStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "alice", false, 64) // loss: -1
	repo.Spin(100, 1, "alice", false, 64) // loss: -1
	repo.Spin(100, 1, "alice", true, 64)  // win: +64

	stats, err := repo.GetPersonalStats(100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Spins != 3 {
		t.Errorf("Spins = %d, want 3", stats.Spins)
	}
	if stats.Wins != 1 {
		t.Errorf("Wins = %d, want 1", stats.Wins)
	}
	if stats.Balance != 62 {
		t.Errorf("Balance = %d, want 62 (-1 -1 +64)", stats.Balance)
	}
}

func TestGetPersonalStats_NoRows(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	_, err := repo.GetPersonalStats(100, 999)
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestGetPersonalStats_Rank(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "rich", true, 64)   // balance: 64
	repo.Spin(100, 2, "mid", false, 64)   // balance: -1
	repo.Spin(100, 3, "poor", false, 64)  // balance: -1
	repo.Spin(100, 3, "poor", false, 64)  // balance: -2

	stats, _ := repo.GetPersonalStats(100, 1)
	if stats.Rank != 1 {
		t.Errorf("rich user rank = %d, want 1", stats.Rank)
	}

	stats, _ = repo.GetPersonalStats(100, 2)
	if stats.Rank != 2 {
		t.Errorf("mid user rank = %d, want 2", stats.Rank)
	}

	stats, _ = repo.GetPersonalStats(100, 3)
	if stats.Rank != 3 {
		t.Errorf("poor user rank = %d, want 3", stats.Rank)
	}
}

func TestGetRichStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "alice", true, 64)  // balance: 64
	repo.Spin(100, 2, "bob", false, 64)   // balance: -1

	stats, err := repo.GetRichStats(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 users, got %d", len(stats))
	}
	if stats[0].Username != "alice" {
		t.Errorf("first = %q, want alice", stats[0].Username)
	}
	if stats[0].Rank != 1 {
		t.Errorf("alice rank = %d, want 1", stats[0].Rank)
	}
	if stats[1].Username != "bob" {
		t.Errorf("second = %q, want bob", stats[1].Username)
	}
}

func TestGetDebtorsStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "alice", true, 64)  // balance: 64
	repo.Spin(100, 2, "bob", false, 64)   // balance: -1

	stats, err := repo.GetDebtorsStats(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 users, got %d", len(stats))
	}
	// Debtors: ascending order, so bob first
	if stats[0].Username != "bob" {
		t.Errorf("first debtor = %q, want bob", stats[0].Username)
	}
	if stats[0].Rank != 1 {
		t.Errorf("bob debtor rank = %d, want 1", stats[0].Rank)
	}
}

func TestGetRichStats_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	stats, err := repo.GetRichStats(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 users, got %d", len(stats))
	}
}

func TestChatIsolation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "alice", true, 64)
	repo.Spin(200, 1, "alice", false, 64)

	stats100, _ := repo.GetPersonalStats(100, 1)
	stats200, _ := repo.GetPersonalStats(200, 1)

	if stats100.Balance != 64 {
		t.Errorf("chat 100 balance = %d, want 64", stats100.Balance)
	}
	if stats200.Balance != -1 {
		t.Errorf("chat 200 balance = %d, want -1", stats200.Balance)
	}
}

func TestSpin_WinStreakTracking(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "alice", true, 64) // win streak: 1
	repo.Spin(100, 1, "alice", true, 64) // win streak: 2
	repo.Spin(100, 1, "alice", true, 64) // win streak: 3

	stats, _ := repo.GetPersonalStats(100, 1)
	if stats.CurrentStreak != 3 {
		t.Errorf("CurrentStreak = %d, want 3", stats.CurrentStreak)
	}
	if stats.MaxStreak != 3 {
		t.Errorf("MaxStreak = %d, want 3", stats.MaxStreak)
	}

	repo.Spin(100, 1, "alice", false, 64) // loss resets win streak

	stats, _ = repo.GetPersonalStats(100, 1)
	if stats.CurrentStreak != 0 {
		t.Errorf("CurrentStreak after loss = %d, want 0", stats.CurrentStreak)
	}
	if stats.MaxStreak != 3 {
		t.Errorf("MaxStreak should remain 3, got %d", stats.MaxStreak)
	}
}

func TestSpin_LossStreakTracking(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "alice", false, 64) // loss streak: 1
	repo.Spin(100, 1, "alice", false, 64) // loss streak: 2
	repo.Spin(100, 1, "alice", false, 64) // loss streak: 3
	repo.Spin(100, 1, "alice", false, 64) // loss streak: 4

	stats, _ := repo.GetPersonalStats(100, 1)
	if stats.CurrentLossStreak != 4 {
		t.Errorf("CurrentLossStreak = %d, want 4", stats.CurrentLossStreak)
	}
	if stats.MaxLossStreak != 4 {
		t.Errorf("MaxLossStreak = %d, want 4", stats.MaxLossStreak)
	}

	repo.Spin(100, 1, "alice", true, 64) // win resets loss streak

	stats, _ = repo.GetPersonalStats(100, 1)
	if stats.CurrentLossStreak != 0 {
		t.Errorf("CurrentLossStreak after win = %d, want 0", stats.CurrentLossStreak)
	}
	if stats.MaxLossStreak != 4 {
		t.Errorf("MaxLossStreak should remain 4, got %d", stats.MaxLossStreak)
	}
}

func TestResetChat(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserStatsRepo(db)

	repo.Spin(100, 1, "alice", true, 64)
	repo.Spin(100, 2, "bob", false, 64)

	err := repo.ResetChat(100)
	if err != nil {
		t.Fatalf("ResetChat() error = %v", err)
	}

	stats, err := repo.GetRichStats(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 users after reset, got %d", len(stats))
	}
}
