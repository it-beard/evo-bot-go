package utils

import (
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// IsUserAdminOrCreator checks if a user is an admin or creator of the specified chat
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
