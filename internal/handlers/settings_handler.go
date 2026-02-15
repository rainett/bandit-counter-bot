package handlers

import (
	"bandit-counter-bot/internal/service"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func GetSettingsCommand(settingsService *service.SettingsService) ext.Handler {
	return handlers.NewCommand("settings", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handleSettingsCommand(b, ctx, settingsService)
	})
}

func handleSettingsCommand(b *gotgbot.Bot, ctx *ext.Context, settingsService *service.SettingsService) error {
	return settingsService.HandleSettingsCommand(b, ctx)
}

func GetPrizeClassicCommand(settingsService *service.SettingsService) ext.Handler {
	return handlers.NewCommand("prize_classic", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handlePrizeClassicCommand(b, ctx, settingsService)
	})
}

func handlePrizeClassicCommand(b *gotgbot.Bot, ctx *ext.Context, settingsService *service.SettingsService) error {
	return settingsService.HandlePrizeClassicCommand(b, ctx)
}

func GetPrizeThreeInARowCommand(settingsService *service.SettingsService) ext.Handler {
	return handlers.NewCommand("prize_three_in_a_row", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handlePrizeThreeInARowCommand(b, ctx, settingsService)
	})
}

func handlePrizeThreeInARowCommand(b *gotgbot.Bot, ctx *ext.Context, settingsService *service.SettingsService) error {
	return settingsService.HandlePrizeThreeInARowCommand(b, ctx)
}

func GetPrizeLemonsCommand(settingsService *service.SettingsService) ext.Handler {
	return handlers.NewCommand("prize_lemons", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handlePrizeLemonsCommand(b, ctx, settingsService)
	})
}

func handlePrizeLemonsCommand(b *gotgbot.Bot, ctx *ext.Context, settingsService *service.SettingsService) error {
	return settingsService.HandlePrizeLemonsCommand(b, ctx)
}
