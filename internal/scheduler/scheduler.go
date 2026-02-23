package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"bandit-counter-bot/internal/cache"
	"bandit-counter-bot/internal/service"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type Scheduler struct {
	cache   *cache.SlotMessageCache
	cleaner *service.MessageCleaner
	bot     *gotgbot.Bot
	loc     *time.Location

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewScheduler(
	cache *cache.SlotMessageCache,
	cleaner *service.MessageCleaner,
	bot *gotgbot.Bot,
	loc *time.Location,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cache:   cache,
		cleaner: cleaner,
		bot:     bot,
		loc:     loc,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.loop()
}

func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
}

func (s *Scheduler) loop() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastCleanupMinute int64 = -1
	var lastReportDay int = -1

	for {
		select {
		case <-s.ctx.Done():
			return

		case nowUTC := <-ticker.C:
			now := nowUTC.In(s.loc)

			minuteKey := now.Unix() / 60
			if (now.Minute() == 0 || now.Minute() == 30) &&
				now.Second() < 30 &&
				minuteKey != lastCleanupMinute {

				lastCleanupMinute = minuteKey
				s.runCleanup()
			}

			// ---------- Daily report: 12:00 ----------
			dayKey := now.YearDay()
			if now.Hour() == 12 &&
				now.Minute() == 0 &&
				now.Second() < 30 &&
				dayKey != lastReportDay {

				lastReportDay = dayKey
				s.runDailyReports()
			}
		}
	}
}

func (s *Scheduler) runCleanup() {
	s.cache.IterateChats(func(chatId int64) bool {
		result := s.cleaner.CleanChat(s.bot, chatId)

		if result.Total == 0 {
			return true
		}

		s.cache.RecordCleanup(chatId, cache.CleanupStats{
			Timestamp:       time.Now().Unix(),
			MessagesDeleted: result.Deleted,
			ErrorsCount:     result.Failed,
		})

		if result.Failed > 0 {
			log.Printf("cleanup for chat %d: deleted %d, failed %d", chatId, result.Deleted, result.Failed)
		}

		return true
	})
}

func (s *Scheduler) runDailyReports() {
	s.cache.IterateChats(func(chatId int64) bool {
		totalDeleted, totalErrors, cycles := s.cache.GetDailyStats(chatId)

		if totalDeleted == 0 {
			return true
		}

		text := formatDailyReport(totalDeleted, totalErrors, cycles)

		_, err := s.bot.SendMessage(chatId, text, nil)
		if err != nil {
			log.Printf("failed to send daily report to chat %d: %v", chatId, err)
		}

		s.cache.ClearDailyStats(chatId)

		return true
	})
}

func formatDailyReport(totalDeleted, totalErrors, cycleCount int) string {
	text := "🧹 Звіт про прибирання за добу\n\n"
	text += formatNumber("Видалено повідомлень", totalDeleted)
	text += formatNumber("Помилок", totalErrors)
	text += formatNumber("Циклів прибирання", cycleCount)
	return text
}

func formatNumber(label string, value int) string {
	return label + ": " + formatWithCommas(value) + "\n"
}

func formatWithCommas(n int) string {
	if n < 1000 {
		return intToString(n)
	}
	s := intToString(n)
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(c)
	}
	return out
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
