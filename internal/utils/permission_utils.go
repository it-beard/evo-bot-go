package utils

import (
	"log"

	"evo-bot-go/internal/config"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func IsUserClubMember(b *gotgbot.Bot, userId int64, config *config.Config) bool {
	chatId := ChatIdToFullChatId(config.SuperGroupChatID)
	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, userId, nil)
	if err != nil {
		log.Printf("Failed to get chat member: %v", err)
		return false
	}

	status := chatMember.GetStatus()
	if status == "left" || status == "kicked" {
		return false
	}

	return true
}

func IsUserAdminOrCreator(b *gotgbot.Bot, userId int64, config *config.Config) bool {
	chatId := ChatIdToFullChatId(config.SuperGroupChatID)

	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, userId, nil)
	if err != nil {
		log.Printf("Failed to get chat member: %v", err)
		return false
	}

	status := chatMember.GetStatus()
	if status == "administrator" || status == "creator" {
		return true
	}
	return false
}
