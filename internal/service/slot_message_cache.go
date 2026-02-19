package service

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type SlotMessage struct {
	MessageId int64
	Timestamp int64
}

type chatData struct {
	mu       sync.Mutex
	messages []SlotMessage
}

type SlotMessageCache struct {
	chats         sync.Map // map[int64]*chatData
	ttl           time.Duration
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

func NewSlotMessageCache() *SlotMessageCache {
	c := &SlotMessageCache{
		chats:       sync.Map{},
		ttl:         24 * time.Hour,
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	c.cleanupTicker = time.NewTicker(1 * time.Hour)
	go c.backgroundCleanup()

	return c
}

func (c *SlotMessageCache) getChatData(chatId int64) *chatData {
	val, _ := c.chats.LoadOrStore(chatId, &chatData{
		messages: make([]SlotMessage, 0),
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

func (c *SlotMessageCache) backgroundCleanup() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.cleanExpiredMessages()
		case <-c.stopCleanup:
			c.cleanupTicker.Stop()
			return
		}
	}
}

func (c *SlotMessageCache) cleanExpiredMessages() {
	now := time.Now().Unix()
	cutoff := now - int64(c.ttl.Seconds())

	c.chats.Range(func(key, value interface{}) bool {
		chatId := key.(int64)
		data := value.(*chatData)

		data.mu.Lock()
		messages := data.messages
		n := 0
		for _, message := range messages {
			if message.Timestamp >= cutoff {
				messages[n] = message
				n++
			}
		}

		if n == 0 {
			data.messages = nil
			data.mu.Unlock()
			c.chats.Delete(chatId)
		} else {
			data.messages = messages[:n]
			data.mu.Unlock()
		}

		return true
	})
}

func (c *SlotMessageCache) CleanForChatId(b *gotgbot.Bot, chatId int64) int {
	val, ok := c.chats.Load(chatId)
	if !ok {
		return 0
	}

	data := val.(*chatData)

	data.mu.Lock()
	toDelete := make([]int64, len(data.messages))
	for i, m := range data.messages {
		toDelete[i] = m.MessageId
	}
	data.messages = nil
	data.mu.Unlock()

	// Remove chat entry from map
	c.chats.Delete(chatId)

	if len(toDelete) == 0 {
		return 0
	}

	const batchSize = 100
	deleted := 0

	for i := 0; i < len(toDelete); i += batchSize {
		end := i + batchSize
		if end > len(toDelete) {
			end = len(toDelete)
		}

		batch := toDelete[i:end]

		ok, err := b.DeleteMessages(chatId, batch, nil)
		if err == nil && ok {
			deleted += len(batch)
		}
	}

	return deleted
}

func (c *SlotMessageCache) Stop() {
	close(c.stopCleanup)
}

func (c *SlotMessageCache) SaveToFile(path string) error {
	// Create snapshot with copy-on-write
	snapshot := make(map[int64][]SlotMessage)

	c.chats.Range(func(key, value interface{}) bool {
		chatId := key.(int64)
		data := value.(*chatData)

		data.mu.Lock()
		// Deep copy messages
		if len(data.messages) > 0 {
			messagesCopy := make([]SlotMessage, len(data.messages))
			copy(messagesCopy, data.messages)
			snapshot[chatId] = messagesCopy
		}
		data.mu.Unlock()

		return true
	})

	// Marshal and write without holding any locks
	jsonData, err := json.Marshal(snapshot)
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

	var data map[int64][]SlotMessage
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	now := time.Now().Unix()
	cutoff := now - int64(c.ttl.Seconds())

	// Filter expired messages and store into sync.Map
	for chatId, messages := range data {
		n := 0
		for _, m := range messages {
			if m.Timestamp >= cutoff {
				messages[n] = m
				n++
			}
		}

		if n > 0 {
			c.chats.Store(chatId, &chatData{
				messages: messages[:n],
			})
		}
	}

	return nil
}

// CountMessages returns the number of messages for a chat (for testing)
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
