package handlers

import (
	"bandit-counter-bot/internal/service"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func GetMeCommand(slotService *service.SlotService) ext.Handler {
	return handlers.NewCommand("me", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handleMeCommand(b, ctx, slotService)
	})
}

func handleMeCommand(b *gotgbot.Bot, ctx *ext.Context, slotService *service.SlotService) error {
	return slotService.HandleMeCommand(b, ctx)
}

func GetRichCommand(slotService *service.SlotService) ext.Handler {
	return handlers.NewCommand("rich", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handleRichCommand(b, ctx, slotService)
	})
}

func handleRichCommand(b *gotgbot.Bot, ctx *ext.Context, slotService *service.SlotService) error {
	return slotService.HandleRichCommand(b, ctx)
}

func GetDebtorsCommand(slotService *service.SlotService) ext.Handler {
	return handlers.NewCommand("debtors", func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handleDebtorsCommand(b, ctx, slotService)
	})
}

func handleDebtorsCommand(b *gotgbot.Bot, ctx *ext.Context, slotService *service.SlotService) error {
	return slotService.HandleDebtorsCommand(b, ctx)
}
