package utils

import (
	"log"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// IsUserAdminOrCreator checks if a user is an admin or creator of the specified chat
func IsUserAdminOrCreator(b *gotgbot.Bot, userId int64, chatId int64) bool {
	formattedChatId, err := strconv.ParseInt("-100"+strconv.FormatInt(chatId, 10), 10, 64)
	if err != nil {
		log.Printf("Failed to parse chat ID: %v", err)
		return false
	}

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
