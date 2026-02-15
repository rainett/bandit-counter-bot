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
