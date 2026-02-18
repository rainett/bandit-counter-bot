package service

import (
	"bandit-counter-bot/internal/repository"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type AuthService struct {
	devIDs       []int64
	settingsRepo *repository.SettingsRepo
}

func NewAuthService(devIDs []int64, settingsRepo *repository.SettingsRepo) *AuthService {
	return &AuthService{devIDs: devIDs, settingsRepo: settingsRepo}
}

func (a *AuthService) IsDev(userId int64) bool {
	for _, id := range a.devIDs {
		if id == userId {
			return true
		}
	}
	return false
}

func (a *AuthService) IsAdmin(b *gotgbot.Bot, chatId int64, userId int64) bool {
	if a.IsDev(userId) {
		return true
	}
	member, err := b.GetChatMember(chatId, userId, nil)
	if err != nil {
		return false
	}
	status := member.GetStatus()
	return status == "creator" || status == "administrator"
}

func (a *AuthService) CanPerform(b *gotgbot.Bot, chatId int64, userId int64, action string) bool {
	if a.IsDev(userId) {
		return true
	}
	member, err := b.GetChatMember(chatId, userId, nil)
	if err != nil {
		return false
	}
	status := member.GetStatus()
	if status == "creator" || status == "administrator" {
		return true
	}
	allowed, err := a.settingsRepo.GetPermission(chatId, action)
	if err != nil {
		return false
	}
	return allowed
}
