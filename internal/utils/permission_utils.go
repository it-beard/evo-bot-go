package utils

import (
	"log"

	"evo-bot-go/internal/config"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func IsUserClubMember(b *gotgbot.Bot, msg *gotgbot.Message, config *config.Config) bool {
	chatId := ChatIdToFullChatId(config.SuperGroupChatID)
	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, msg.From.Id, nil)
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

func IsUserAdminOrCreator(b *gotgbot.Bot, userId int64, chatId int64) bool {
	formattedChatId := ChatIdToFullChatId(chatId)

	// Check if user is member of target group
	chatMember, err := b.GetChatMember(formattedChatId, userId, nil)
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
