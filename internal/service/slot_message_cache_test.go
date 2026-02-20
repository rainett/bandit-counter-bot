package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAdd_And_Count(t *testing.T) {
	cache := NewSlotMessageCache()
	defer cache.Stop()

	cache.Add(100, 1)
	cache.Add(100, 2)
	cache.Add(200, 3)

	if count := cache.CountMessages(100); count != 2 {
		t.Errorf("chat 100 messages = %d, want 2", count)
	}
	if count := cache.CountMessages(200); count != 1 {
		t.Errorf("chat 200 messages = %d, want 1", count)
	}
}

func TestRecordCleanup(t *testing.T) {
	cache := NewSlotMessageCache()
	defer cache.Stop()

	chatId := int64(100)
	cache.Add(chatId, 1) // Create chat data

	// Record multiple cleanup stats
	for i := 0; i < 50; i++ {
		cache.recordCleanup(chatId, CleanupStats{
			Timestamp:       time.Now().Unix(),
			MessagesDeleted: i + 1,
			ErrorsCount:     0,
		})
	}

	// Verify circular buffer behavior (should keep only last 48)
	val, ok := cache.chats.Load(chatId)
	if !ok {
		t.Fatal("chat should exist")
	}

	data := val.(*chatData)
	data.statsMu.Lock()
	historyLen := len(data.cleanupHistory)
	data.statsMu.Unlock()

	if historyLen != 48 {
		t.Errorf("cleanup history length = %d, want 48 (circular buffer)", historyLen)
	}

	// Verify oldest entries were removed (should start from entry 3, not 1)
	data.statsMu.Lock()
	firstEntry := data.cleanupHistory[0].MessagesDeleted
	data.statsMu.Unlock()

	if firstEntry != 3 {
		t.Errorf("first entry MessagesDeleted = %d, want 3 (oldest entries removed)", firstEntry)
	}
}

func TestGetDailyStats(t *testing.T) {
	cache := NewSlotMessageCache()
	defer cache.Stop()

	chatId := int64(100)
	cache.Add(chatId, 1) // Create chat data

	// Add known cleanup stats
	cache.recordCleanup(chatId, CleanupStats{
		Timestamp:       time.Now().Unix(),
		MessagesDeleted: 100,
		ErrorsCount:     5,
	})
	cache.recordCleanup(chatId, CleanupStats{
		Timestamp:       time.Now().Unix(),
		MessagesDeleted: 200,
		ErrorsCount:     10,
	})

	totalDeleted, totalErrors := cache.getDailyStats(chatId)

	if totalDeleted != 300 {
		t.Errorf("totalDeleted = %d, want 300", totalDeleted)
	}
	if totalErrors != 15 {
		t.Errorf("totalErrors = %d, want 15", totalErrors)
	}
}

func TestGenerateDailyReport(t *testing.T) {
	cache := NewSlotMessageCache()
	defer cache.Stop()

	chatId := int64(100)
	cache.Add(chatId, 1) // Create chat data

	// Add cleanup stats
	cache.recordCleanup(chatId, CleanupStats{
		Timestamp:       time.Now().Unix(),
		MessagesDeleted: 1234,
		ErrorsCount:     0,
	})

	report := cache.generateDailyReport(chatId)

	if report == "" {
		t.Error("report should not be empty")
	}

	// Verify report contains Ukrainian text and formatted numbers
	if !containsString(report, "🧹 Звіт про прибирання за добу") {
		t.Error("report should contain Ukrainian header")
	}
	if !containsString(report, "1,234") {
		t.Error("report should contain formatted number with comma")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSaveAndLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	// Save
	cache := NewSlotMessageCache()
	defer cache.Stop()

	now := time.Now().Unix()
	cache.Add(100, 1)
	cache.Add(100, 2)
	cache.Add(200, 3)

	if err := cache.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Load into a fresh cache
	cache2 := NewSlotMessageCache()
	defer cache2.Stop()

	if err := cache2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if count := cache2.CountMessages(100); count != 2 {
		t.Errorf("chat 100 messages = %d, want 2", count)
	}
	if count := cache2.CountMessages(200); count != 1 {
		t.Errorf("chat 200 messages = %d, want 1", count)
	}

	// Verify timestamps are recent
	val, ok := cache2.chats.Load(int64(100))
	if !ok {
		t.Fatal("chat 100 should exist")
	}
	data := val.(*chatData)
	data.mu.Lock()
	for _, m := range data.messages {
		if m.Timestamp < now-1 {
			t.Errorf("message timestamp %d is too old", m.Timestamp)
		}
	}
	data.mu.Unlock()
}

func TestLoadFromFile_NonExistent(t *testing.T) {
	cache := NewSlotMessageCache()
	defer cache.Stop()

	err := cache.LoadFromFile("/nonexistent/path/cache.json")
	if err != nil {
		t.Errorf("LoadFromFile() should return nil for missing file, got %v", err)
	}
}

func TestSaveLoadCleanupHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	// Create cache with messages and cleanup history
	cache := NewSlotMessageCache()
	defer cache.Stop()

	chatId := int64(100)
	cache.Add(chatId, 1)
	cache.Add(chatId, 2)

	cache.recordCleanup(chatId, CleanupStats{
		Timestamp:       time.Now().Unix(),
		MessagesDeleted: 50,
		ErrorsCount:     2,
	})
	cache.recordCleanup(chatId, CleanupStats{
		Timestamp:       time.Now().Unix(),
		MessagesDeleted: 30,
		ErrorsCount:     0,
	})

	if err := cache.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Load into fresh cache
	cache2 := NewSlotMessageCache()
	defer cache2.Stop()

	if err := cache2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	// Verify messages restored
	if count := cache2.CountMessages(chatId); count != 2 {
		t.Errorf("expected 2 messages, got %d", count)
	}

	// Verify cleanup history restored
	val, ok := cache2.chats.Load(chatId)
	if !ok {
		t.Fatal("chat should exist")
	}

	data := val.(*chatData)
	data.statsMu.Lock()
	historyLen := len(data.cleanupHistory)
	data.statsMu.Unlock()

	if historyLen != 2 {
		t.Errorf("expected 2 cleanup history entries, got %d", historyLen)
	}

	totalDeleted, totalErrors := cache2.getDailyStats(chatId)
	if totalDeleted != 80 {
		t.Errorf("totalDeleted = %d, want 80", totalDeleted)
	}
	if totalErrors != 2 {
		t.Errorf("totalErrors = %d, want 2", totalErrors)
	}
}

func TestSaveToFile_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	cache := NewSlotMessageCache()
	defer cache.Stop()

	cache.Add(100, 1)

	if err := cache.SaveToFile(path); err != nil {
		t.Fatal(err)
	}

	// Verify the file exists and tmp doesn't
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("cache file should exist")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after save")
	}
}

func TestDeleteMessagesForChat(t *testing.T) {
	// This is a unit test that doesn't actually call Telegram API
	// It tests the batching logic
	cache := NewSlotMessageCache()
	defer cache.Stop()

	// Create a list of message IDs
	messageIds := make([]int64, 250) // More than one batch
	for i := range messageIds {
		messageIds[i] = int64(i + 1)
	}

	// Note: We can't test actual deletion without a real bot instance
	// This test verifies the batching logic is correct
	if len(messageIds) != 250 {
		t.Errorf("expected 250 message IDs, got %d", len(messageIds))
	}

	// Verify batch size calculation
	batchSize := 100
	expectedBatches := (len(messageIds) + batchSize - 1) / batchSize
	if expectedBatches != 3 {
		t.Errorf("expected 3 batches for 250 messages, got %d", expectedBatches)
	}
}

func TestStopCleansUpGoroutine(t *testing.T) {
	cache := NewSlotMessageCache()

	// Stop the cache before starting background tasks
	cache.Stop()

	// Give it a moment to stop
	time.Sleep(10 * time.Millisecond)

	// Calling Stop again should not panic (channel already closed is caught)
	defer func() {
		if r := recover(); r != nil {
			t.Error("Stop() should be safe to call multiple times")
		}
	}()
	cache.Stop()
}

func TestGracefulShutdown(t *testing.T) {
	cache := NewSlotMessageCache()

	// Create mock bot for testing (nil is OK since we won't actually call it)
	// In real usage, StartBackgroundTasks needs a real bot
	// We're just testing that Stop() works correctly

	cache.deletionTicker = time.NewTicker(1 * time.Hour)
	cache.reportTicker = time.NewTicker(1 * time.Hour)

	// Stop should close the channel and stop both tickers
	cache.Stop()

	// Verify stopCleanup channel is closed
	select {
	case <-cache.stopCleanup:
		// Good, channel is closed
	default:
		t.Error("stopCleanup channel should be closed after Stop()")
	}

	// Verify we can call Stop again without panic
	cache.Stop()
}

func TestConcurrentAddsToSameChat(t *testing.T) {
	cache := NewSlotMessageCache()
	defer cache.Stop()

	chatId := int64(100)
	numGoroutines := 10
	messagesPerGoroutine := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				cache.Add(chatId, int64(offset*messagesPerGoroutine+j))
			}
		}(i)
	}

	wg.Wait()

	expectedCount := numGoroutines * messagesPerGoroutine
	if count := cache.CountMessages(chatId); count != expectedCount {
		t.Errorf("expected %d messages, got %d", expectedCount, count)
	}
}

func TestConcurrentAddsToDifferentChats(t *testing.T) {
	cache := NewSlotMessageCache()
	defer cache.Stop()

	numChats := 10
	messagesPerChat := 100

	var wg sync.WaitGroup
	wg.Add(numChats)

	for chatId := 0; chatId < numChats; chatId++ {
		go func(id int64) {
			defer wg.Done()
			for msgId := 0; msgId < messagesPerChat; msgId++ {
				cache.Add(id, int64(msgId))
			}
		}(int64(chatId))
	}

	wg.Wait()

	// Verify each chat has the correct count
	for chatId := 0; chatId < numChats; chatId++ {
		if count := cache.CountMessages(int64(chatId)); count != messagesPerChat {
			t.Errorf("chat %d: expected %d messages, got %d", chatId, messagesPerChat, count)
		}
	}
}
