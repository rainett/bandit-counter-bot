package service

import (
	"bandit-counter-bot/internal/repository"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type ResetService struct {
	statsRepo *repository.UserStatsRepo
	auth      *AuthService
}

func NewResetService(statsRepo *repository.UserStatsRepo, auth *AuthService) *ResetService {
	return &ResetService{statsRepo: statsRepo, auth: auth}
}

func (s *ResetService) HandleResetCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	chatId := ctx.EffectiveMessage.Chat.Id
	userId := ctx.EffectiveMessage.From.Id

	if !s.auth.CanPerform(b, chatId, userId, "reset") {
		_, _ = ctx.EffectiveMessage.Reply(b, "а фіг тобі", &gotgbot.SendMessageOpts{})
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
	userId := cb.From.Id

	if !s.auth.CanPerform(b, chatId, userId, "reset") {
		cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "потерпиш",
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
					{Text: "Вйо", CallbackData: "reset:step3"},
					{Text: "Нєє", CallbackData: "reset:cancel"},
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
					{Text: "Погнали", CallbackData: "reset:confirm"},
					{Text: "Я манав", CallbackData: "reset:cancel"},
				},
			},
		}
		cb.Message.EditText(b, "🚨🚨🚨 ШЕ РАЗ ПИТАЮ І ВСЬО", &gotgbot.EditMessageTextOpts{
			ReplyMarkup: keyboard,
		})

	case "confirm":
		if err := s.statsRepo.ResetChat(chatId); err != nil {
			cb.Answer(b, nil)
			return err
		}
		cb.Message.EditText(b, "💥 пацани, не знаю вашє хто ви", &gotgbot.EditMessageTextOpts{})

	case "cancel":
		cb.Message.EditText(b, "❌ отмінет", &gotgbot.EditMessageTextOpts{})
	}

	cb.Answer(b, nil)
	return nil
}
