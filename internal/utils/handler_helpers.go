package utils

import (
	"log"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// CheckAdminPermissions checks if the user has admin permissions and returns an appropriate error response
// Returns true if user has permission, false otherwise
func CheckAdminPermissions(b *gotgbot.Bot, ctx *ext.Context, superGroupChatID int64, commandName string) bool {
	msg := ctx.EffectiveMessage

	if !IsUserAdminOrCreator(b, msg.From.Id, superGroupChatID) {
		if _, err := msg.Reply(b, "Эта команда доступна только администраторам.", nil); err != nil {
			log.Printf("Failed to send admin-only message: %v", err)
		}
		log.Printf("User %d tried to use %s without admin rights", msg.From.Id, commandName)
		return false
	}

	return true
}

// CheckPrivateChatType checks if the command is used in a private chat and returns an appropriate error response
// Returns true if used in private chat, false otherwise
func CheckPrivateChatType(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage

	if msg.Chat.Type != constants.PrivateChatType {
		if _, err := msg.Reply(b, "Эта команда доступна только в личном чате.", nil); err != nil {
			log.Printf("Failed to send private-only message: %v", err)
		}
		return false
	}

	return true
}

func CheckClubMemberPermissions(b *gotgbot.Bot, msg *gotgbot.Message, config *config.Config, commandName string) bool {
	if !IsUserClubMember(b, msg, config) {
		if _, err := msg.Reply(b, "Эта команда доступна только участникам клуба.", nil); err != nil {
			log.Printf("Failed to send club-only message: %v", err)
		}
		log.Printf("User %d tried to use %s without club member rights", msg.From.Id, commandName)
		return false
	}

	return true
}

// SendLoggedReply sends a reply to the user with proper logging
func SendLoggedReply(b *gotgbot.Bot, msg *gotgbot.Message, text string, err error) {
	if _, replyErr := msg.Reply(b, text, nil); replyErr != nil {
		log.Printf("Failed to send error message: %v", replyErr)
	}
	if err != nil {
		log.Printf("Error: %v", err)
	}
}

// SendLoggedReply sends a reply to the user with proper logging
func SendLoggedMarkdownReply(b *gotgbot.Bot, msg *gotgbot.Message, text string, err error) {
	if _, replyErr := msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: "Markdown",
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
	}); replyErr != nil {
		log.Printf("Failed to send error message: %v", replyErr)
	}
	if err != nil {
		log.Printf("Error: %v", err)
	}
}

// SendLoggedReply sends a reply to the user with proper logging
func SendLoggedHtmlReply(b *gotgbot.Bot, msg *gotgbot.Message, text string, err error) {
	if _, replyErr := msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
	}); replyErr != nil {
		log.Printf("Failed to send error message: %v", replyErr)
	}
	if err != nil {
		log.Printf("Error: %v", err)
	}
}

// CheckAdminAndPrivateChat combines permission and chat type checking for admin-only private commands
// Returns true if all checks pass, false otherwise
func CheckAdminAndPrivateChat(b *gotgbot.Bot, ctx *ext.Context, superGroupChatID int64, commandName string) bool {
	if !CheckAdminPermissions(b, ctx, superGroupChatID, commandName) {
		return false
	}

	if !CheckPrivateChatType(b, ctx) {
		return false
	}

	return true
}
