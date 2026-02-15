package handlers

import (
	"bandit-counter-bot/internal/service"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

func GetSlotHandler(slotService *service.SlotService) ext.Handler {
	return handlers.NewMessage(message.Dice, func(b *gotgbot.Bot, ctx *ext.Context) error {
		return handleSlot(ctx, slotService)
	})
}

func handleSlot(ctx *ext.Context, slotService *service.SlotService) error {
	if ctx.EffectiveMessage.Dice.Emoji == "🎰" && ctx.Message.ForwardOrigin == nil {
		return slotService.HandleSlot(ctx)
	}
	return nil
}
