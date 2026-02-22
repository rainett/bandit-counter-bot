package cache

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"
)

type SlotMessage struct {
	MessageId int64 `json:"message_id"`
	Timestamp int64 `json:"timestamp"`
}

type CleanupStats struct {
	Timestamp       int64 `json:"timestamp"`
	MessagesDeleted int   `json:"messages_deleted"`
	ErrorsCount     int   `json:"errors_count"`
}

type chatData struct {
	mu             sync.Mutex
	messages       []SlotMessage
	statsMu        sync.Mutex
	cleanupHistory []CleanupStats // circular buffer, up to 48 entries
}

type SlotMessageCache struct {
	chats sync.Map // map[int64]*chatData
}

// NewSlotMessageCache returns ready cache
func NewSlotMessageCache() *SlotMessageCache {
	return &SlotMessageCache{}
}

func (c *SlotMessageCache) getChatData(chatId int64) *chatData {
	val, _ := c.chats.LoadOrStore(chatId, &chatData{
		messages:       make([]SlotMessage, 0),
		cleanupHistory: make([]CleanupStats, 0, 48),
	})
	return val.(*chatData)
}

func (c *SlotMessageCache) Add(chatId, messageId int64) {
	data := c.getChatData(chatId)
	data.mu.Lock()
	defer data.mu.Unlock()

	now := time.Now().Unix()
	data.messages = append(data.messages, SlotMessage{
		MessageId: messageId,
		Timestamp: now,
	})
}

func (c *SlotMessageCache) CountMessages(chatId int64) int {
	val, ok := c.chats.Load(chatId)
	if !ok {
		return 0
	}
	data := val.(*chatData)
	data.mu.Lock()
	defer data.mu.Unlock()
	return len(data.messages)
}

// DrainForDeletion atomically takes all messages for chat and clears the slice.
// Caller is responsible to requeue failed IDs if needed.
func (c *SlotMessageCache) DrainForDeletion(chatId int64) []int64 {
	val, ok := c.chats.Load(chatId)
	if !ok {
		return nil
	}
	data := val.(*chatData)

	data.mu.Lock()
	defer data.mu.Unlock()

	if len(data.messages) == 0 {
		return nil
	}
	out := make([]int64, len(data.messages))
	for i, m := range data.messages {
		out[i] = m.MessageId
	}
	data.messages = nil
	return out
}

// RequeueFailed appends failed message IDs back to chat's message list with current timestamp.
func (c *SlotMessageCache) RequeueFailed(chatId int64, failed []int64) {
	if len(failed) == 0 {
		return
	}
	data := c.getChatData(chatId)
	data.mu.Lock()
	defer data.mu.Unlock()

	now := time.Now().Unix()
	for _, id := range failed {
		data.messages = append(data.messages, SlotMessage{MessageId: id, Timestamp: now})
	}
}

// RecordCleanup records single cleanup run for chat (circular buffer of last 48 runs)
func (c *SlotMessageCache) RecordCleanup(chatId int64, stats CleanupStats) {
	val, ok := c.chats.Load(chatId)
	if !ok {
		// If chat missing, create it but only with cleanup history
		d := c.getChatData(chatId)
		d.statsMu.Lock()
		d.cleanupHistory = append(d.cleanupHistory, stats)
		d.statsMu.Unlock()
		return
	}
	data := val.(*chatData)
	data.statsMu.Lock()
	defer data.statsMu.Unlock()

	if len(data.cleanupHistory) >= 48 {
		copy(data.cleanupHistory, data.cleanupHistory[1:])
		data.cleanupHistory[47] = stats
	} else {
		data.cleanupHistory = append(data.cleanupHistory, stats)
	}
}

// GetDailyStats aggregates all entries in the circular buffer (should represent ~24h if scheduler runs every 30m)
func (c *SlotMessageCache) GetDailyStats(chatId int64) (totalDeleted, totalErrors int, cycleCount int) {
	val, ok := c.chats.Load(chatId)
	if !ok {
		return 0, 0, 0
	}
	data := val.(*chatData)
	data.statsMu.Lock()
	defer data.statsMu.Unlock()
	cycleCount = len(data.cleanupHistory)
	for _, s := range data.cleanupHistory {
		totalDeleted += s.MessagesDeleted
		totalErrors += s.ErrorsCount
	}
	return totalDeleted, totalErrors, cycleCount
}

// IterateChats calls fn for each chat. If fn returns false, iteration stops.
func (c *SlotMessageCache) IterateChats(fn func(chatId int64) bool) {
	c.chats.Range(func(key, value interface{}) bool {
		chatId := key.(int64)
		return fn(chatId)
	})
}

// SaveToFile persists cache and cleanup history. keys are strings in JSON.
type persistentCache struct {
	Messages       map[string][]SlotMessage  `json:"messages"`
	CleanupHistory map[string][]CleanupStats `json:"cleanup_history"`
}

func (c *SlotMessageCache) SaveToFile(path string) error {
	snap := persistentCache{
		Messages:       make(map[string][]SlotMessage),
		CleanupHistory: make(map[string][]CleanupStats),
	}

	c.chats.Range(func(key, value interface{}) bool {
		chatId := key.(int64)
		data := value.(*chatData)

		data.mu.Lock()
		if len(data.messages) > 0 {
			copyMsgs := make([]SlotMessage, len(data.messages))
			copy(copyMsgs, data.messages)
			snap.Messages[strconv.FormatInt(chatId, 10)] = copyMsgs
		}
		data.mu.Unlock()

		data.statsMu.Lock()
		if len(data.cleanupHistory) > 0 {
			copyHist := make([]CleanupStats, len(data.cleanupHistory))
			copy(copyHist, data.cleanupHistory)
			snap.CleanupHistory[strconv.FormatInt(chatId, 10)] = copyHist
		}
		data.statsMu.Unlock()

		return true
	})

	jsonData, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, jsonData, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *SlotMessageCache) LoadFromFile(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var snap persistentCache
	if err := json.Unmarshal(bytes, &snap); err == nil {
		for k, msgs := range snap.Messages {
			id, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				continue
			}
			c.chats.Store(id, &chatData{
				messages:       msgs,
				cleanupHistory: snap.CleanupHistory[k],
			})
		}
		return nil
	}

	// Fallback: try old format map[string][]SlotMessage
	var old map[string][]SlotMessage
	if err := json.Unmarshal(bytes, &old); err != nil {
		return err
	}
	for k, msgs := range old {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		c.chats.Store(id, &chatData{
			messages:       msgs,
			cleanupHistory: make([]CleanupStats, 0, 48),
		})
	}
	return nil
}

func (c *SlotMessageCache) DeleteChat(chatId int64) {
	c.chats.Delete(chatId)
}
