package service

import (
	"bandit-counter-bot/internal/repository"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type SettingsService struct {
	repo *repository.SettingsRepo
}

func (s *SettingsService) HandleSettingsCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	text := "🎰Налаштування крутілки\n\n" +
		"/prize_classic - 777\n" +
		"/prize_three_in_a_row - будь-які три в ряд\n" +
		"/prize_lemons - лимони"
	ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{})
	return nil
}

func NewSettingsService(repo *repository.SettingsRepo) *SettingsService {
	return &SettingsService{repo: repo}
}
