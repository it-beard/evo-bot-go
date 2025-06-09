package services

import (
	"evo-bot-go/internal/database/repositories"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type ProfileService struct {
	bot *gotgbot.Bot
}

func NewProfileService(bot *gotgbot.Bot) *ProfileService {
	return &ProfileService{
		bot: bot,
	}
}

// IsProfileComplete checks if a profile has the minimum required fields for publishing
func (s *ProfileService) IsProfileComplete(user *repositories.User, profile *repositories.Profile) bool {
	if user == nil || profile == nil {
		return false
	}

	if user.Firstname == "" {
		return false
	}

	if user.Lastname == "" {
		return false
	}

	if profile.Bio == "" {
		return false
	}

	return true
}
