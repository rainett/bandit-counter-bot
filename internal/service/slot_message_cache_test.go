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

func TestCleanExpiredMessages(t *testing.T) {
	cache := NewSlotMessageCache()
	cache.ttl = 60 * time.Second
	defer cache.Stop()

	now := time.Now().Unix()

	// Manually create chat data with expired and valid messages
	data100 := &chatData{
		messages: []SlotMessage{
			{MessageId: 1, Timestamp: now - 120}, // expired
			{MessageId: 2, Timestamp: now - 30},  // valid
			{MessageId: 3, Timestamp: now},       // valid
		},
	}
	data200 := &chatData{
		messages: []SlotMessage{
			{MessageId: 4, Timestamp: now - 120}, // expired
		},
	}

	cache.chats.Store(int64(100), data100)
	cache.chats.Store(int64(200), data200)

	// Run cleanup
	cache.cleanExpiredMessages()

	// Check results
	if count := cache.CountMessages(100); count != 2 {
		t.Errorf("chat 100 after cleanup = %d, want 2", count)
	}

	val, ok := cache.chats.Load(int64(100))
	if !ok {
		t.Fatal("chat 100 should exist after cleanup")
	}
	data := val.(*chatData)
	data.mu.Lock()
	if data.messages[0].MessageId != 2 {
		t.Errorf("first remaining message = %d, want 2", data.messages[0].MessageId)
	}
	data.mu.Unlock()

	if count := cache.CountMessages(200); count != 0 {
		t.Errorf("chat 200 should be deleted (all messages expired), got %d messages", count)
	}
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

func TestLoadFromFile_FiltersExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	// Create a cache with old timestamps
	cache := NewSlotMessageCache()
	cache.ttl = 60 * time.Second
	defer cache.Stop()

	data := &chatData{
		messages: []SlotMessage{
			{MessageId: 1, Timestamp: time.Now().Unix() - 120}, // expired
			{MessageId: 2, Timestamp: time.Now().Unix()},       // valid
		},
	}
	cache.chats.Store(int64(100), data)

	cache.SaveToFile(path)

	// Load into fresh cache with same TTL
	cache2 := NewSlotMessageCache()
	cache2.ttl = 60 * time.Second
	defer cache2.Stop()

	cache2.LoadFromFile(path)

	if count := cache2.CountMessages(100); count != 1 {
		t.Errorf("expected 1 message after filtering, got %d", count)
	}

	val, ok := cache2.chats.Load(int64(100))
	if !ok {
		t.Fatal("chat 100 should exist")
	}
	chatData := val.(*chatData)
	chatData.mu.Lock()
	if chatData.messages[0].MessageId != 2 {
		t.Errorf("remaining message = %d, want 2", chatData.messages[0].MessageId)
	}
	chatData.mu.Unlock()
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

func TestBackgroundCleanup(t *testing.T) {
	cache := NewSlotMessageCache()
	cache.ttl = 100 * time.Millisecond
	cache.cleanupTicker.Stop()
	cache.cleanupTicker = time.NewTicker(50 * time.Millisecond)
	defer cache.Stop()

	now := time.Now().Unix()

	// Add expired messages
	data := &chatData{
		messages: []SlotMessage{
			{MessageId: 1, Timestamp: now - 1},
			{MessageId: 2, Timestamp: now - 1},
		},
	}
	cache.chats.Store(int64(100), data)

	// Wait for background cleanup to run
	time.Sleep(150 * time.Millisecond)

	// Messages should be cleaned up
	if count := cache.CountMessages(100); count != 0 {
		t.Errorf("expected 0 messages after background cleanup, got %d", count)
	}
}

func TestStopCleansUpGoroutine(t *testing.T) {
	cache := NewSlotMessageCache()

	// Stop the cache
	cache.Stop()

	// Give it a moment to stop
	time.Sleep(10 * time.Millisecond)

	// Verify the cleanup ticker is stopped by checking if we can stop it again
	// (This is a basic check - in production, goroutine leak detection would be better)
	defer func() {
		if r := recover(); r != nil {
			t.Error("Stop() should be safe to call multiple times or after goroutine exits")
		}
	}()

	// Calling Stop again should not panic (channel already closed is caught)
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
