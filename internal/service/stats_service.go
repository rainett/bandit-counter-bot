package service

import (
	"bandit-counter-bot/internal/domain"
	"bandit-counter-bot/internal/repository"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const statsPageSize = 10

type StatsService struct {
	statsRepo *repository.UserStatsRepo
}

func NewStatsService(statsRepo *repository.UserStatsRepo) *StatsService {
	return &StatsService{statsRepo: statsRepo}
}

func (s *StatsService) HandleStatsCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	chatId := ctx.EffectiveMessage.Chat.Id
	text, keyboard, err := s.buildStatsMessage(chatId, "rich", 0)
	if err != nil {
		return err
	}
	_, _ = ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
		ReplyMarkup: keyboard,
	})
	return nil
}

func (s *StatsService) HandleStatsCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 {
		cb.Answer(b, nil)
		return nil
	}

	view := parts[1]
	page, err := strconv.Atoi(parts[2])
	if err != nil {
		page = 0
	}

	chatId := cb.Message.GetChat().Id
	text, keyboard, err := s.buildStatsMessage(chatId, view, page)
	if err != nil {
		cb.Answer(b, nil)
		return err
	}

	_, _, _ = cb.Message.EditText(b, text, &gotgbot.EditMessageTextOpts{
		ReplyMarkup: keyboard,
	})
	cb.Answer(b, nil)
	return nil
}

func (s *StatsService) buildStatsMessage(chatId int64, view string, page int) (string, gotgbot.InlineKeyboardMarkup, error) {
	var stats []domain.RatingStats
	var err error
	var title string

	switch view {
	case "rich":
		stats, err = s.statsRepo.GetRichStats(chatId)
		title = "🎩 Багатії"
	case "debtors":
		stats, err = s.statsRepo.GetDebtorsStats(chatId)
		title = "🧙 Боржники"
	case "lucky":
		stats, err = s.statsRepo.GetLuckyStats(chatId)
		title = "🍀 Удачливі"
	case "streaks":
		stats, err = s.statsRepo.GetStreakStats(chatId)
		title = "🔥 Серії"
	default:
		stats, err = s.statsRepo.GetRichStats(chatId)
		view = "rich"
		title = "🎩 Багатії"
	}
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	totalPages := int(math.Ceil(float64(len(stats)) / float64(statsPageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * statsPageSize
	end := start + statsPageSize
	if end > len(stats) {
		end = len(stats)
	}

	var builder strings.Builder
	builder.WriteString(title + "\n\n")

	if len(stats) == 0 {
		builder.WriteString("порожняк")
	} else {
		pageStats := stats[start:end]
		for _, u := range pageStats {
			switch view {
			case "lucky":
				fmt.Fprintf(&builder, "%d. 👤 %s — 🍀 %.1f%%, 🎰 %d, 🍾 %d\n",
					u.Rank, u.Username, u.Luck, u.Spins, u.Wins)
			case "streaks":
				fmt.Fprintf(&builder, "%d. 👤 %s — 🔥 %d, 💀 %d, 🎰 %d\n",
					u.Rank, u.Username, u.MaxStreak, u.MaxLossStreak, u.Spins)
			default:
				fmt.Fprintf(&builder, "%d. 👤 %s — 💸 %d, 🎰 %d, 🍾 %d\n",
					u.Rank, u.Username, u.Balance, u.Spins, u.Wins)
			}
		}
	}

	if totalPages > 1 {
		fmt.Fprintf(&builder, "\nСторінка %d/%d", page+1, totalPages)
	}

	keyboard := buildStatsKeyboard(view, page, totalPages)
	return builder.String(), keyboard, nil
}

func buildStatsKeyboard(activeView string, page, totalPages int) gotgbot.InlineKeyboardMarkup {
	viewRows := [][]struct {
		key   string
		label string
	}{
		{{"rich", "Багатії"}, {"debtors", "Боржники"}},
		{{"lucky", "Удачливі"}, {"streaks", "Серії"}},
	}

	var rows [][]gotgbot.InlineKeyboardButton
	for _, row := range viewRows {
		var buttons []gotgbot.InlineKeyboardButton
		for _, v := range row {
			label := v.label
			if v.key == activeView {
				label = "✅ " + label
			}
			buttons = append(buttons, gotgbot.InlineKeyboardButton{
				Text:         label,
				CallbackData: fmt.Sprintf("stats:%s:0", v.key),
			})
		}
		rows = append(rows, buttons)
	}

	if totalPages > 1 {
		var navButtons []gotgbot.InlineKeyboardButton
		if page > 0 {
			navButtons = append(navButtons, gotgbot.InlineKeyboardButton{
				Text:         "⬅️ Назад",
				CallbackData: fmt.Sprintf("stats:%s:%d", activeView, page-1),
			})
		}
		if page < totalPages-1 {
			navButtons = append(navButtons, gotgbot.InlineKeyboardButton{
				Text:         "Далі ➡️",
				CallbackData: fmt.Sprintf("stats:%s:%d", activeView, page+1),
			})
		}
		if len(navButtons) > 0 {
			rows = append(rows, navButtons)
		}
	}

	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}
