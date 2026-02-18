package service

import (
	"bandit-counter-bot/internal/repository"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type SettingsService struct {
	repo *repository.SettingsRepo
}

func NewSettingsService(repo *repository.SettingsRepo) *SettingsService {
	return &SettingsService{repo: repo}
}

func (s *SettingsService) HandleSettingsCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	text := "🎰Налаштування крутілки\n\n" +
		"/prize_classic - 777\n" +
		"/prize_three_in_a_row - будь-які три в ряд\n" +
		"/prize_lemons - лимони"
	ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{})
	return nil
}

func (s *SettingsService) HandlePrizeClassicCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	values := "[64]"
	return s.updatePrize(values, b, ctx)
}

func (s *SettingsService) HandlePrizeThreeInARowCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	values := "[1,22,43,64]"
	return s.updatePrize(values, b, ctx)
}

func (s *SettingsService) HandlePrizeLemonsCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	values := "[43]"
	return s.updatePrize(values, b, ctx)
}

func (s *SettingsService) updatePrize(values string, b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	if msg.Chat.Type == "private" {
		msg.Reply(b, "налаштування працюють тільки в групах", &gotgbot.SendMessageOpts{})
		return nil
	}

	member, err := b.GetChatMember(msg.Chat.Id, msg.From.Id, nil)
	if err != nil {
		return err
	}

	status := member.GetStatus()
	if status != "creator" && status != "administrator" {
		msg.Reply(b, "тільки адміни можуть змінювати налаштування", &gotgbot.SendMessageOpts{})
		return nil
	}

	chatId := msg.Chat.Id
	if err := s.repo.UpdatePrizeValues(values, chatId); err != nil {
		return err
	}
	msg.Reply(b, "Оновлено умову виграшу", &gotgbot.SendMessageOpts{})
	return nil
}
