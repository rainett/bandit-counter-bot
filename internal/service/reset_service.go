package service

import (
	"bandit-counter-bot/internal/repository"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type ResetService struct {
	statsRepo *repository.UserStatsRepo
}

func NewResetService(statsRepo *repository.UserStatsRepo) *ResetService {
	return &ResetService{statsRepo: statsRepo}
}

func (s *ResetService) HandleResetCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	if !s.isAdmin(b, ctx.EffectiveMessage.Chat.Id, ctx.EffectiveMessage.From.Id) {
		_, _ = ctx.EffectiveMessage.Reply(b, "Тільки адміни можуть скидати статистику", &gotgbot.SendMessageOpts{})
		return nil
	}

	keyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "Так", CallbackData: "reset:step2"},
				{Text: "Ні", CallbackData: "reset:cancel"},
			},
		},
	}
	_, _ = ctx.EffectiveMessage.Reply(b, "⚠️ Ти впевнений що хочеш скинути ВСЮ статистику?", &gotgbot.SendMessageOpts{
		ReplyMarkup: keyboard,
	})
	return nil
}

func (s *ResetService) HandleResetCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	chatId := cb.Message.GetChat().Id

	if !s.isAdmin(b, chatId, cb.From.Id) {
		cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Тільки адміни можуть це робити",
		})
		return nil
	}

	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		cb.Answer(b, nil)
		return nil
	}
	action := parts[1]

	switch action {
	case "step2":
		keyboard := gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{Text: "Впевнений", CallbackData: "reset:step3"},
					{Text: "Та ні", CallbackData: "reset:cancel"},
				},
			},
		}
		cb.Message.EditText(b, "⚠️⚠️ Це видалить статистику ВСІХ гравців у цьому чаті. Точно?", &gotgbot.EditMessageTextOpts{
			ReplyMarkup: keyboard,
		})

	case "step3":
		keyboard := gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{Text: "🔥 ЗРОБИТИ ЦЕ 🔥", CallbackData: "reset:confirm"},
					{Text: "Я передумав", CallbackData: "reset:cancel"},
				},
			},
		}
		cb.Message.EditText(b, "🚨🚨🚨 ОСТАННІЙ ШАНС! Назад дороги нема!", &gotgbot.EditMessageTextOpts{
			ReplyMarkup: keyboard,
		})

	case "confirm":
		if err := s.statsRepo.ResetChat(chatId); err != nil {
			cb.Answer(b, nil)
			return err
		}
		cb.Message.EditText(b, "💥 Статистику стерто з лиця землі. Починаємо з нуля!", &gotgbot.EditMessageTextOpts{})

	case "cancel":
		cb.Message.EditText(b, "❌ Скинення скасовано. Фух!", &gotgbot.EditMessageTextOpts{})
	}

	cb.Answer(b, nil)
	return nil
}

func (s *ResetService) isAdmin(b *gotgbot.Bot, chatId int64, userId int64) bool {
	member, err := b.GetChatMember(chatId, userId, nil)
	if err != nil {
		return false
	}
	status := member.GetStatus()
	return status == "creator" || status == "administrator"
}
