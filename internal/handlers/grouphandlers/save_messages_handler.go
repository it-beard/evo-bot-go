package grouphandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type SaveMessagesHandler struct {
	saveUpdateMessageService *services.SaveUpdateMessageService
	config                   *config.Config
	bot                      *gotgbot.Bot
}

func NewSaveMessagesHandler(
	saveUpdateMessageService *services.SaveUpdateMessageService,
	config *config.Config,
	bot *gotgbot.Bot,
) ext.Handler {
	h := &SaveMessagesHandler{
		saveUpdateMessageService: saveUpdateMessageService,
		config:                   config,
		bot:                      bot,
	}
	return handlers.NewMessage(h.check, h.handle).SetAllowEdited(true)
}

func (h *SaveMessagesHandler) check(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}

	// Skip private chats
	if msg.Chat.Type == constants.PrivateChatType {
		return false
	}

	// Skip forum topic created or edited messages (handled by SaveTopicsHandler)
	if msg.ForumTopicCreated != nil || msg.ForumTopicEdited != nil {
		return false
	}

	// Skip messages from GroupAnonymousBot or admin
	// and equal "update" or "delete" (from AdminMessageControlHandler)
	if (msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" ||
		utils.IsUserAdminOrCreator(h.bot, msg.From.Id, h.config)) &&
		(msg.Text == constants.AdminMessageControlUpdateCommand ||
			msg.Text == constants.AdminMessageControlDeleteCommand) {
		return false
	}

	// Check if this is a regular message with content
	return msg.Text != "" || msg.Caption != "" || msg.Voice != nil || msg.Audio != nil ||
		msg.Document != nil || msg.Photo != nil || msg.Video != nil || msg.VideoNote != nil ||
		msg.Sticker != nil || msg.Animation != nil
}

func (h *SaveMessagesHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	// Check if this is an edited message
	if ctx.Update.EditedMessage != nil {
		if h.isMessageForDeletion(ctx.Update.EditedMessage.Text) {
			return h.saveUpdateMessageService.Delete(ctx.Update.EditedMessage)
		} else {
			return h.saveUpdateMessageService.SaveOrUpdate(ctx.Update.EditedMessage)
		}
	}

	// Handle regular new messages
	msg := ctx.EffectiveMessage
	return h.saveUpdateMessageService.Save(msg)
}

func (s *SaveMessagesHandler) isMessageForDeletion(messageText string) bool {
	return messageText == constants.SaveMessagesDeleteEnCommand ||
		messageText == constants.SaveMessagesDeleteRuCommand
}
