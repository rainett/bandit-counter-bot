package service

import (
	"bandit-counter-bot/internal/repository"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type SlotService struct {
	statsRepo    *repository.UserStatsRepo
	settingsRepo *repository.SettingsRepo
	messageCache *SlotMessageCache
}

func NewSlotService(userRepo *repository.UserStatsRepo, settingsRepo *repository.SettingsRepo, messageCache *SlotMessageCache) *SlotService {
	return &SlotService{statsRepo: userRepo, settingsRepo: settingsRepo, messageCache: messageCache}
}

func (s *SlotService) HandleSlot(ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	value := int(msg.Dice.Value)

	prizeValues, err := s.settingsRepo.GetPrizeValues(msg.Chat.Id)
	if err != nil {
		return err
	}

	var balanceDelta int64 = -1
	var winDelta int64 = 0
	var win = false
	for _, v := range prizeValues {
		if value == v {
			balanceDelta = 64
			winDelta = 1
			win = true
			break
		}
	}
	if !win {
		s.messageCache.Add(msg.Chat.Id, msg.MessageId)
	}
	return s.statsRepo.Spin(msg.Chat.Id, msg.From.Id, msg.From.FirstName, winDelta, balanceDelta)
}

func (s *SlotService) HandleMeCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	chatId := ctx.EffectiveMessage.Chat.Id
	userId := ctx.EffectiveMessage.From.Id
	stats, err := s.statsRepo.GetPersonalStats(chatId, userId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.EffectiveMessage.Reply(b, "ти хто ваше", &gotgbot.SendMessageOpts{})
			return nil
		}
		return err
	}
	text := fmt.Sprintf("🎰 Прокрутів: %d\n🍾 Виграшів: %d\n💸 Баланс: %d\n⭐ Місце в чаті: %d",
		stats.Spins, stats.Wins, stats.Balance, stats.Rank)
	_, _ = ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{})
	return nil
}

func (s *SlotService) HandleRichCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	chatId := ctx.EffectiveMessage.Chat.Id
	stats, err := s.statsRepo.GetRichStats(chatId)
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		ctx.EffectiveMessage.Reply(b, "порожняк", &gotgbot.SendMessageOpts{})
		return nil
	}

	var builder strings.Builder
	builder.WriteString("🎩Топ гравців:\n\n")

	for _, u := range stats {
		fmt.Fprintf(
			&builder,
			"%d. 👤 %s — 💸 %d, 🎰 %d, 🍾 %d\n",
			u.Rank,
			u.Username,
			u.Balance,
			u.Spins,
			u.Wins,
		)
	}
	ctx.EffectiveMessage.Reply(b, builder.String(), &gotgbot.SendMessageOpts{})
	return nil
}

func (s *SlotService) HandleDebtorsCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	chatId := ctx.EffectiveMessage.Chat.Id
	stats, err := s.statsRepo.GetDebtorsStats(chatId)
	if err != nil {
		return err
	}

	if len(stats) == 0 {
		ctx.EffectiveMessage.Reply(b, "порожняк", &gotgbot.SendMessageOpts{})
		return nil
	}

	var builder strings.Builder
	builder.WriteString("🧙Топ боржників:\n\n")

	for _, u := range stats {
		fmt.Fprintf(
			&builder,
			"%d️. 👤 %s — 💸 %d, 🎰 %d, 🍾 %d\n",
			u.Rank,
			u.Username,
			u.Balance,
			u.Spins,
			u.Wins,
		)
	}

	ctx.EffectiveMessage.Reply(b, builder.String(), &gotgbot.SendMessageOpts{})
	return nil
}

func (s *SlotService) HandleCleanCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	cleanedMessagesCount := s.messageCache.CleanForChatId(b, ctx.Message.Chat.Id)
	text := fmt.Sprintf("🧹Очищено повідомлень: %d", cleanedMessagesCount)
	ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{})
	return nil
}
