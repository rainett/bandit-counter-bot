package service

import (
	"bandit-counter-bot/internal/cache"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type CleanResult struct {
	Deleted int
	Failed  int
	Total   int
}

type MessageCleaner struct {
	cache *cache.SlotMessageCache
}

func NewMessageCleaner(cache *cache.SlotMessageCache) *MessageCleaner {
	return &MessageCleaner{cache: cache}
}

func (c *MessageCleaner) CleanChat(
	b *gotgbot.Bot,
	chatId int64,
) CleanResult {
	messageIds := c.cache.DrainForDeletion(chatId)
	if len(messageIds) == 0 {
		return CleanResult{}
	}

	const batchSize = 100
	deleted := 0
	failed := make([]int64, 0)

	for i := 0; i < len(messageIds); i += batchSize {
		end := i + batchSize
		if end > len(messageIds) {
			end = len(messageIds)
		}

		batch := messageIds[i:end]
		ok, err := b.DeleteMessages(chatId, batch, nil)
		if err == nil && ok {
			deleted += len(batch)
		} else {
			failed = append(failed, batch...)
		}
	}

	if len(failed) > 0 {
		c.cache.RequeueFailed(chatId, failed)
	}

	if deleted == 0 && len(failed) == len(messageIds) {
		c.cache.DeleteChat(chatId)
	}

	return CleanResult{
		Deleted: deleted,
		Failed:  len(failed),
		Total:   len(messageIds),
	}
}
