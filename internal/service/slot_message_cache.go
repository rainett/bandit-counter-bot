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

type SlotMessageCache struct {
	mu         sync.Mutex
	cache      map[int64][]SlotMessage
	addCount   uint64
	cleanEvery uint64
	ttl        int64
}

func NewSlotMessageCache() *SlotMessageCache {
	return &SlotMessageCache{
		mu:         sync.Mutex{},
		cache:      make(map[int64][]SlotMessage),
		cleanEvery: 1000,
		ttl:        int64(24 * time.Hour / time.Second),
	}
}

func (c *SlotMessageCache) Add(chatId, messageId int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().Unix()
	c.cache[chatId] = append(c.cache[chatId], SlotMessage{
		MessageId: messageId,
		Timestamp: now,
	})

	c.addCount++

	if c.addCount%c.cleanEvery == 0 {
		c.clearOldCache(now)
	}
}

func (c *SlotMessageCache) clearOldCache(now int64) {
	cutoff := now - c.ttl

	for chatId, messages := range c.cache {
		if len(messages) == 0 {
			continue
		}

		n := 0
		for _, message := range messages {
			if message.Timestamp >= cutoff {
				messages[n] = message
				n++
			}
		}

		if n == 0 {
			delete(c.cache, chatId)
		} else {
			c.cache[chatId] = messages[:n]
		}
	}
}

func (c *SlotMessageCache) CleanForChatId(b *gotgbot.Bot, chatId int64) int {
	c.clearOldCache(time.Now().Unix())
	var toDelete []int64

	c.mu.Lock()
	messages, ok := c.cache[chatId]
	if ok {
		toDelete = make([]int64, 0, len(messages))
		for _, m := range messages {
			toDelete = append(toDelete, m.MessageId)
		}
		delete(c.cache, chatId)
	}
	c.mu.Unlock()

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

func (c *SlotMessageCache) SaveToFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(c.cache)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
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
	cutoff := now - c.ttl

	for chatID, messages := range data {
		n := 0
		for _, m := range messages {
			if m.Timestamp >= cutoff {
				messages[n] = m
				n++
			}
		}
		if n == 0 {
			delete(data, chatID)
		} else {
			data[chatID] = messages[:n]
		}
	}

	c.mu.Lock()
	c.cache = data
	c.mu.Unlock()

	return nil
}
