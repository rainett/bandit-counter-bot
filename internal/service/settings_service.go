package service

import (
	"bandit-counter-bot/internal/repository"
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var prizeModes = []struct {
	key    string
	label  string
	values string
}{
	{"classic", "777", "[64]"},
	{"three_in_a_row", "Три в ряд", "[1,22,43,64]"},
	{"lemons", "Лимони", "[43]"},
}

var winAmounts = []int64{32, 64, 128, 256}

type SettingsService struct {
	repo *repository.SettingsRepo
	auth *AuthService
}

func NewSettingsService(repo *repository.SettingsRepo, auth *AuthService) *SettingsService {
	return &SettingsService{repo: repo, auth: auth}
}

func (s *SettingsService) HandleSettingsCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	chatId := ctx.EffectiveMessage.Chat.Id
	userId := ctx.EffectiveMessage.From.Id
	isAdmin := s.auth.IsAdmin(b, chatId, userId)
	text, keyboard, err := s.buildSettingsMessage(chatId, isAdmin)
	if err != nil {
		return err
	}
	_, _ = ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
		ReplyMarkup: keyboard,
	})
	return nil
}

func (s *SettingsService) HandleSettingsCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 {
		cb.Answer(b, nil)
		return nil
	}

	chatId := cb.Message.GetChat().Id
	userId := cb.From.Id
	category := parts[1]
	value := parts[2]

	switch category {
	case "prize", "amount":
		if !s.auth.CanPerform(b, chatId, userId, "settings") {
			cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
				Text: "нізя тобі таке клацать",
			})
			return nil
		}
		if category == "prize" {
			for _, mode := range prizeModes {
				if mode.key == value {
					if err := s.repo.UpdatePrizeValues(mode.values, chatId); err != nil {
						cb.Answer(b, nil)
						return err
					}
					break
				}
			}
		} else {
			amount, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				cb.Answer(b, nil)
				return nil
			}
			if err := s.repo.UpdateWinAmount(amount, chatId); err != nil {
				cb.Answer(b, nil)
				return err
			}
		}

	case "perm":
		if !s.auth.IsAdmin(b, chatId, userId) {
			cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
				Text: "Тільки адміни можуть змінювати дозволи",
			})
			return nil
		}
		if _, err := s.repo.TogglePermission(chatId, value); err != nil {
			cb.Answer(b, nil)
			return err
		}

	default:
		cb.Answer(b, nil)
		return nil
	}

	isAdmin := s.auth.IsAdmin(b, chatId, userId)
	text, keyboard, err := s.buildSettingsMessage(chatId, isAdmin)
	if err != nil {
		cb.Answer(b, nil)
		return err
	}

	_, _, _ = cb.Message.EditText(b, text, &gotgbot.EditMessageTextOpts{
		ReplyMarkup: keyboard,
	})
	cb.Answer(b, nil)
	return nil
}

func (s *SettingsService) buildSettingsMessage(chatId int64, isAdmin bool) (string, gotgbot.InlineKeyboardMarkup, error) {
	currentMode, err := s.repo.GetPrizeMode(chatId)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	currentAmount, err := s.repo.GetWinAmount(chatId)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	modeLabel := "777"
	for _, m := range prizeModes {
		if m.key == currentMode {
			modeLabel = m.label
			break
		}
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "🎰 Налаштування крутілки\n\nРежим виграшу: %s\nСума виграшу: %d", modeLabel, currentAmount)

	var prizeButtons []gotgbot.InlineKeyboardButton
	for _, m := range prizeModes {
		label := m.label
		if m.key == currentMode {
			label = "✅ " + label
		}
		prizeButtons = append(prizeButtons, gotgbot.InlineKeyboardButton{
			Text:         label,
			CallbackData: fmt.Sprintf("settings:prize:%s", m.key),
		})
	}

	var amountButtons []gotgbot.InlineKeyboardButton
	for _, a := range winAmounts {
		label := fmt.Sprintf("%d", a)
		if a == currentAmount {
			label = "✅ " + label
		}
		amountButtons = append(amountButtons, gotgbot.InlineKeyboardButton{
			Text:         label,
			CallbackData: fmt.Sprintf("settings:amount:%d", a),
		})
	}

	rows := [][]gotgbot.InlineKeyboardButton{
		prizeButtons,
		amountButtons,
	}

	if isAdmin {
		allowSettings, _ := s.repo.GetPermission(chatId, "settings")
		allowReset, _ := s.repo.GetPermission(chatId, "reset")

		settingsLabel := "🔒 Налаштування"
		if allowSettings {
			settingsLabel = "🔓 Налаштування"
		}
		resetLabel := "🔒 Скидання"
		if allowReset {
			resetLabel = "🔓 Скидання"
		}

		settingsStatus := "адміни"
		if allowSettings {
			settingsStatus = "всі"
		}
		resetStatus := "адміни"
		if allowReset {
			resetStatus = "всі"
		}

		fmt.Fprintf(&builder, "\n\n🔐 Дозволи\nНалаштування: %s | Скидання: %s", settingsStatus, resetStatus)

		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: settingsLabel, CallbackData: "settings:perm:settings"},
			{Text: resetLabel, CallbackData: "settings:perm:reset"},
		})
	}

	keyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
	return builder.String(), keyboard, nil
}
