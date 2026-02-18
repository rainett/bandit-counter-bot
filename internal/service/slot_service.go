package service

import (
	"bandit-counter-bot/internal/repository"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"

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

func (s *SlotService) HandleSlot(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	value := int(msg.Dice.Value)

	prizeValues, err := s.settingsRepo.GetPrizeValues(msg.Chat.Id)
	if err != nil {
		return err
	}

	winAmount, err := s.settingsRepo.GetWinAmount(msg.Chat.Id)
	if err != nil {
		return err
	}

	win := false
	for _, v := range prizeValues {
		if value == v {
			win = true
			break
		}
	}
	if !win {
		s.messageCache.Add(msg.Chat.Id, msg.MessageId)
	}
	if win {
		s.sendWinReaction(b, msg)
	}
	return s.statsRepo.Spin(msg.Chat.Id, msg.From.Id, msg.From.FirstName, win, winAmount)
}

var winReactionEmojis = []string{"🎉", "🔥", "❤", "👍", "🏆", "⚡", "🍾", "👏", "🤩", "😍"}

func (s *SlotService) sendWinReaction(b *gotgbot.Bot, msg *gotgbot.Message) {
	emoji := winReactionEmojis[rand.Intn(len(winReactionEmojis))]
	msg.SetReaction(b, &gotgbot.SetMessageReactionOpts{
		Reaction: []gotgbot.ReactionType{&gotgbot.ReactionTypeEmoji{Emoji: emoji}},
		IsBig:    true,
	})
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
	text := fmt.Sprintf(
		"🎰 Прокрутів: %d\n🍾 Виграшів: %d\n💸 Баланс: %d\n⭐ Місце в чаті: %d\n🍀 Удача: %.1f%%\n🔥 Поточна серія: %d\n🏆 Макс серія: %d",
		stats.Spins, stats.Wins, stats.Balance, stats.Rank, stats.Luck, stats.CurrentStreak, stats.MaxStreak)
	_, _ = ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{})
	return nil
}

func (s *SlotService) HandleCleanCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	cleanedMessagesCount := s.messageCache.CleanForChatId(b, ctx.Message.Chat.Id)
	var text = "нема шо чистити"
	if cleanedMessagesCount != 0 {
		text = fmt.Sprintf("🧹Очищено повідомлень: %d", cleanedMessagesCount)
	}
	ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{})
	return nil
}

func (s *SlotService) HandleHelpCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	text := "🎰 Доступні команди:\n\n" +
		"/me - моя статистика\n" +
		"/stats - рейтинг гравців\n" +
		"/settings - налаштування крутілки\n" +
		"/reset - скинути статистику чату\n" +
		"/clean - видалити програшні повідомлення\n" +
		"/help - список команд"
	_, _ = ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{})
	return nil
}
