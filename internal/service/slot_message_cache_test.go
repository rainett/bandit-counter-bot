package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdd_And_Count(t *testing.T) {
	cache := NewSlotMessageCache()

	cache.Add(100, 1)
	cache.Add(100, 2)
	cache.Add(200, 3)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if len(cache.cache[100]) != 2 {
		t.Errorf("chat 100 messages = %d, want 2", len(cache.cache[100]))
	}
	if len(cache.cache[200]) != 1 {
		t.Errorf("chat 200 messages = %d, want 1", len(cache.cache[200]))
	}
}

func TestClearOldCache(t *testing.T) {
	cache := NewSlotMessageCache()
	cache.ttl = 60 // 60 seconds

	now := time.Now().Unix()

	cache.mu.Lock()
	cache.cache[100] = []SlotMessage{
		{MessageId: 1, Timestamp: now - 120}, // expired
		{MessageId: 2, Timestamp: now - 30},  // valid
		{MessageId: 3, Timestamp: now},        // valid
	}
	cache.cache[200] = []SlotMessage{
		{MessageId: 4, Timestamp: now - 120}, // expired
	}
	cache.clearOldCache(now)
	cache.mu.Unlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if len(cache.cache[100]) != 2 {
		t.Errorf("chat 100 after cleanup = %d, want 2", len(cache.cache[100]))
	}
	if cache.cache[100][0].MessageId != 2 {
		t.Errorf("first remaining message = %d, want 2", cache.cache[100][0].MessageId)
	}
	if _, exists := cache.cache[200]; exists {
		t.Error("chat 200 should be deleted (all messages expired)")
	}
}

func TestSaveAndLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	// Save
	cache := NewSlotMessageCache()
	now := time.Now().Unix()
	cache.Add(100, 1)
	cache.Add(100, 2)
	cache.Add(200, 3)

	if err := cache.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Load into a fresh cache
	cache2 := NewSlotMessageCache()
	if err := cache2.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	cache2.mu.Lock()
	defer cache2.mu.Unlock()

	if len(cache2.cache[100]) != 2 {
		t.Errorf("chat 100 messages = %d, want 2", len(cache2.cache[100]))
	}
	if len(cache2.cache[200]) != 1 {
		t.Errorf("chat 200 messages = %d, want 1", len(cache2.cache[200]))
	}

	// Verify timestamps are recent
	for _, m := range cache2.cache[100] {
		if m.Timestamp < now-1 {
			t.Errorf("message timestamp %d is too old", m.Timestamp)
		}
	}
}

func TestLoadFromFile_NonExistent(t *testing.T) {
	cache := NewSlotMessageCache()
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
	cache.ttl = 60

	cache.mu.Lock()
	cache.cache[100] = []SlotMessage{
		{MessageId: 1, Timestamp: time.Now().Unix() - 120}, // expired
		{MessageId: 2, Timestamp: time.Now().Unix()},       // valid
	}
	cache.mu.Unlock()

	cache.SaveToFile(path)

	// Load into fresh cache with same TTL
	cache2 := NewSlotMessageCache()
	cache2.ttl = 60
	cache2.LoadFromFile(path)

	cache2.mu.Lock()
	defer cache2.mu.Unlock()

	if len(cache2.cache[100]) != 1 {
		t.Errorf("expected 1 message after filtering, got %d", len(cache2.cache[100]))
	}
	if cache2.cache[100][0].MessageId != 2 {
		t.Errorf("remaining message = %d, want 2", cache2.cache[100][0].MessageId)
	}
}

func TestSaveToFile_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	cache := NewSlotMessageCache()
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

func TestAdd_PeriodicCleanup(t *testing.T) {
	cache := NewSlotMessageCache()
	cache.cleanEvery = 5
	cache.ttl = 1

	// Add 4 messages with old timestamps
	cache.mu.Lock()
	cache.cache[100] = []SlotMessage{
		{MessageId: 1, Timestamp: time.Now().Unix() - 10},
		{MessageId: 2, Timestamp: time.Now().Unix() - 10},
		{MessageId: 3, Timestamp: time.Now().Unix() - 10},
		{MessageId: 4, Timestamp: time.Now().Unix() - 10},
	}
	cache.addCount = 4
	cache.mu.Unlock()

	// This should trigger cleanup (addCount becomes 5)
	cache.Add(200, 5)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if _, exists := cache.cache[100]; exists {
		t.Error("expired messages in chat 100 should have been cleaned up")
	}
	if len(cache.cache[200]) != 1 {
		t.Errorf("chat 200 should have 1 message, got %d", len(cache.cache[200]))
	}
}
