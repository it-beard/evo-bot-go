package utils

import (
	"log"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// ChatMemberGetter defines the interface for getting chat member information.
// This allows for mocking in tests.
type ChatMemberGetter interface {
	GetChatMember(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error)
}

func IsUserClubMember(b ChatMemberGetter, userId int64, config *config.Config) bool {
	chatId := ChatIdToFullChatId(config.SuperGroupChatID)
	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, userId, nil)
	if err != nil {
		log.Printf("Failed to get chat member: %v", err)
		return false
	}

	return IsChatMemberInClub(chatMember)
}

// IsChatMemberInClub reports whether a chat member of the club's supergroup is currently in it
func IsChatMemberInClub(chatMember gotgbot.ChatMember) bool {
	status := chatMember.GetStatus()
	if status == "left" || status == "kicked" {
		return false
	}

	// Restrictions survive leaving the chat, so a restricted user is a member only while is_member is set
	if status == "restricted" && !chatMember.MergeChatMember().IsMember {
		return false
	}

	return true
}

func IsUserAdminOrCreator(b ChatMemberGetter, userId int64, config *config.Config) bool {
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

func IsMessageFromSuperGroupChat(chat gotgbot.Chat) bool {
	return chat.Type == constants.SuperGroupChatType
}

// IsClubSuperGroupChat reports whether the chat is the club's supergroup from the config.
// The bot can be added to any other group, so group updates must be checked against it before
// anything is stored or changed.
func IsClubSuperGroupChat(chat gotgbot.Chat, config *config.Config) bool {
	return IsMessageFromSuperGroupChat(chat) && chat.Id == ChatIdToFullChatId(config.SuperGroupChatID)
}
