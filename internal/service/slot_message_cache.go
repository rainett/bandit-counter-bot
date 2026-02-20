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

type CleanupStats struct {
	Timestamp       int64 // Unix timestamp of cleanup
	MessagesDeleted int   // Messages deleted in this run
	ErrorsCount     int   // Errors encountered
}

type chatData struct {
	mu       sync.Mutex
	messages []SlotMessage

	// Per-chat statistics
	statsMu        sync.Mutex     // Protects cleanupHistory
	cleanupHistory []CleanupStats // Last 48 cleanup runs (24 hours)
}

type SlotMessageCache struct {
	chats          sync.Map // map[int64]*chatData
	stopCleanup    chan struct{}
	deletionTicker *time.Ticker // 30-minute combined cleanup (Telegram + cache)
	reportTicker   *time.Ticker // 24-hour daily reporting
}

func NewSlotMessageCache() *SlotMessageCache {
	c := &SlotMessageCache{
		chats:       sync.Map{},
		stopCleanup: make(chan struct{}),
	}
	return c
}

// StartBackgroundTasks starts the background cleanup and reporting goroutines
func (c *SlotMessageCache) StartBackgroundTasks(b *gotgbot.Bot) {
	c.deletionTicker = time.NewTicker(30 * time.Minute)
	c.reportTicker = time.NewTicker(24 * time.Hour)

	go c.backgroundAutoDeletion(b)
	go c.backgroundReporting(b)
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

func (c *SlotMessageCache) backgroundAutoDeletion(b *gotgbot.Bot) {
	for {
		select {
		case <-c.deletionTicker.C:
			c.performAutoDeletion(b)
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *SlotMessageCache) performAutoDeletion(b *gotgbot.Bot) {
	c.chats.Range(func(key, value interface{}) bool {
		chatId := key.(int64)
		data := value.(*chatData)

		// Copy message IDs and clear cache
		data.mu.Lock()
		toDelete := make([]int64, len(data.messages))
		for i, m := range data.messages {
			toDelete[i] = m.MessageId
		}
		data.messages = nil
		data.mu.Unlock()

		if len(toDelete) == 0 {
			return true // Continue to next chat
		}

		// Delete from Telegram
		deleted, errors := c.deleteMessagesForChat(b, chatId, toDelete)

		// Record statistics
		c.recordCleanup(chatId, CleanupStats{
			Timestamp:       time.Now().Unix(),
			MessagesDeleted: deleted,
			ErrorsCount:     errors,
		})

		// If chat is completely inaccessible, remove it
		if deleted == 0 && errors == len(toDelete) {
			c.chats.Delete(chatId)
		}

		return true
	})
}

func (c *SlotMessageCache) backgroundReporting(b *gotgbot.Bot) {
	for {
		select {
		case <-c.reportTicker.C:
			c.sendDailyReportsToAllChats(b)
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *SlotMessageCache) sendDailyReportsToAllChats(b *gotgbot.Bot) {
	c.chats.Range(func(key, value interface{}) bool {
		chatId := key.(int64)
		data := value.(*chatData)

		// Check if chat has any cleanup history
		data.statsMu.Lock()
		hasHistory := len(data.cleanupHistory) > 0
		data.statsMu.Unlock()

		if !hasHistory {
			return true
		}

		// Generate and send report
		report := c.generateDailyReport(chatId)
		if report != "" {
			_, err := b.SendMessage(chatId, report, nil)
			if err != nil {
				// Log but don't crash - will try again in 24 hours
				// Bot might not have send permissions
			}
		}

		return true
	})
}

func (c *SlotMessageCache) deleteMessagesForChat(b *gotgbot.Bot, chatId int64, messageIds []int64) (deleted, errors int) {
	const batchSize = 100

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
			errors += len(batch)
		}
	}

	return deleted, errors
}

func (c *SlotMessageCache) recordCleanup(chatId int64, stats CleanupStats) {
	val, ok := c.chats.Load(chatId)
	if !ok {
		return
	}

	data := val.(*chatData)
	data.statsMu.Lock()
	defer data.statsMu.Unlock()

	// Circular buffer: keep last 48 entries
	if len(data.cleanupHistory) >= 48 {
		// Shift left and replace last element
		copy(data.cleanupHistory, data.cleanupHistory[1:])
		data.cleanupHistory[47] = stats
	} else {
		data.cleanupHistory = append(data.cleanupHistory, stats)
	}
}

func (c *SlotMessageCache) getDailyStats(chatId int64) (totalDeleted, totalErrors int) {
	val, ok := c.chats.Load(chatId)
	if !ok {
		return 0, 0
	}

	data := val.(*chatData)
	data.statsMu.Lock()
	defer data.statsMu.Unlock()

	// Aggregate last 24 hours of stats
	for _, stat := range data.cleanupHistory {
		totalDeleted += stat.MessagesDeleted
		totalErrors += stat.ErrorsCount
	}

	return totalDeleted, totalErrors
}

func (c *SlotMessageCache) generateDailyReport(chatId int64) string {
	totalDeleted, totalErrors := c.getDailyStats(chatId)

	val, ok := c.chats.Load(chatId)
	if !ok {
		return ""
	}

	data := val.(*chatData)
	data.statsMu.Lock()
	cycleCount := len(data.cleanupHistory)
	data.statsMu.Unlock()

	if totalDeleted == 0 && totalErrors == 0 {
		return ""
	}

	return formatDailyReport(totalDeleted, totalErrors, cycleCount)
}

func formatDailyReport(totalDeleted, totalErrors, cycleCount int) string {
	report := "🧹 Звіт про прибирання за добу\n\n"
	report += formatNumber("Видалено повідомлень", totalDeleted)
	report += formatNumber("Помилок", totalErrors)
	report += formatNumber("Циклів прибирання", cycleCount)
	return report
}

func formatNumber(label string, value int) string {
	return label + ": " + formatWithCommas(value) + "\n"
}

func formatWithCommas(n int) string {
	if n < 1000 {
		return intToString(n)
	}

	// Simple comma formatting for thousands
	s := intToString(n)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}

	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
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
	// Signal all goroutines to stop
	select {
	case <-c.stopCleanup:
		return // Already stopped
	default:
		close(c.stopCleanup)
	}

	// Stop both tickers
	if c.deletionTicker != nil {
		c.deletionTicker.Stop()
	}
	if c.reportTicker != nil {
		c.reportTicker.Stop()
	}
}

type PersistentCache struct {
	Messages       map[int64][]SlotMessage       `json:"messages"`
	CleanupHistory map[int64][]CleanupStats      `json:"cleanup_history"`
}

func (c *SlotMessageCache) SaveToFile(path string) error {
	// Create snapshot with copy-on-write
	snapshot := PersistentCache{
		Messages:       make(map[int64][]SlotMessage),
		CleanupHistory: make(map[int64][]CleanupStats),
	}

	c.chats.Range(func(key, value interface{}) bool {
		chatId := key.(int64)
		data := value.(*chatData)

		data.mu.Lock()
		// Deep copy messages
		if len(data.messages) > 0 {
			messagesCopy := make([]SlotMessage, len(data.messages))
			copy(messagesCopy, data.messages)
			snapshot.Messages[chatId] = messagesCopy
		}
		data.mu.Unlock()

		data.statsMu.Lock()
		// Deep copy cleanup history
		if len(data.cleanupHistory) > 0 {
			historyCopy := make([]CleanupStats, len(data.cleanupHistory))
			copy(historyCopy, data.cleanupHistory)
			snapshot.CleanupHistory[chatId] = historyCopy
		}
		data.statsMu.Unlock()

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

	// Try new format first
	var persistentCache PersistentCache
	if err := json.Unmarshal(bytes, &persistentCache); err == nil && persistentCache.Messages != nil {
		// New format with cleanup history
		for chatId, messages := range persistentCache.Messages {
			if len(messages) > 0 {
				history := persistentCache.CleanupHistory[chatId]
				c.chats.Store(chatId, &chatData{
					messages:       messages,
					cleanupHistory: history,
				})
			}
		}
		return nil
	}

	// Fall back to old format (just messages map)
	var oldData map[int64][]SlotMessage
	if err := json.Unmarshal(bytes, &oldData); err != nil {
		return err
	}

	for chatId, messages := range oldData {
		if len(messages) > 0 {
			c.chats.Store(chatId, &chatData{
				messages:       messages,
				cleanupHistory: make([]CleanupStats, 0, 48),
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
