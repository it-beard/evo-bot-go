package grouphandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"log"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type AdminMessageControlHandler struct {
	saveUpdateMessageService *services.SaveUpdateMessageService
	permissionsService       *services.PermissionsService
	config                   *config.Config
	bot                      *gotgbot.Bot
	messageSenderService     *services.MessageSenderService
}

func NewAdminMessageControlHandler(
	saveUpdateMessageService *services.SaveUpdateMessageService,
	permissionsService *services.PermissionsService,
	config *config.Config,
	bot *gotgbot.Bot,
	messageSenderService *services.MessageSenderService,
) ext.Handler {
	h := &AdminMessageControlHandler{
		saveUpdateMessageService: saveUpdateMessageService,
		permissionsService:       permissionsService,
		config:                   config,
		bot:                      bot,
		messageSenderService:     messageSenderService,
	}
	return handlers.NewMessage(h.check, h.handle)
}

func (h *AdminMessageControlHandler) check(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}

	// Skip private chats
	if msg.Chat.Type == constants.PrivateChatType {
		return false
	}

	// Must be a reply to another message
	if msg.ReplyToMessage == nil {
		return false
	}

	// Must be from an admin or GroupAnonymousBot
	if !utils.IsUserAdminOrCreator(h.bot, msg.From.Id, h.config) &&
		(msg.From.IsBot && msg.From.Username != "GroupAnonymousBot") {
		return false
	}

	// Must be "update" or "delete" command
	return msg.Text == constants.AdminMessageControlUpdateCommand ||
		msg.Text == constants.AdminMessageControlDeleteCommand
}

func (h *AdminMessageControlHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	command := strings.ToLower(strings.TrimSpace(msg.Text))
	repliedMessage := msg.ReplyToMessage

	switch command {
	case constants.AdminMessageControlUpdateCommand:
		return h.handleUpdateCommand(msg, repliedMessage)
	case constants.AdminMessageControlDeleteCommand:
		return h.handleDeleteCommand(msg, repliedMessage)
	default:
		return nil // Should not reach here due to check() method
	}
}

// handleUpdateCommand processes the "update" command
func (h *AdminMessageControlHandler) handleUpdateCommand(adminMsg *gotgbot.Message, repliedMessage *gotgbot.Message) error {
	// Try to save or update the replied message
	err := h.saveUpdateMessageService.SaveOrUpdate(repliedMessage)
	if err != nil {
		log.Printf("%s: Failed to save/update message %d: %v",
			utils.GetCurrentTypeName(), repliedMessage.MessageId, err)

		// Reply to admin with error
		_, replyErr := h.messageSenderService.SendWithReturnMessage(h.getAdminUserID(adminMsg), "❌ Не удалось обновить сообщение в базе данных.", nil)
		if replyErr != nil {
			log.Printf("%s: Failed to reply to admin: %v", utils.GetCurrentTypeName(), replyErr)
		}
		return err
	}

	// Then delete the admin's command message
	_, err = adminMsg.Delete(h.bot, nil)
	if err != nil {
		log.Printf("%s: Failed to delete admin command message %d: %v",
			utils.GetCurrentTypeName(), adminMsg.MessageId, err)
	}

	// Send success message to admin DM
	_, err = h.messageSenderService.SendWithReturnMessage(h.getAdminUserID(adminMsg), "✅ Сообщение успешно добавлено/обновлено в базе данных.", nil)
	if err != nil {
		log.Printf("%s: Failed to reply to admin: %v", utils.GetCurrentTypeName(), err)
	}

	return nil
}

// handleDeleteCommand processes the "delete" command
func (h *AdminMessageControlHandler) handleDeleteCommand(adminMsg *gotgbot.Message, repliedMessage *gotgbot.Message) error {

	// First, try to delete the replied message from Telegram and database
	err := h.saveUpdateMessageService.Delete(repliedMessage)
	if err != nil {
		log.Printf("%s: Failed to delete replied message %d: %v",
			utils.GetCurrentTypeName(), repliedMessage.MessageId, err)
	}

	// Then delete the admin's command message
	_, err = adminMsg.Delete(h.bot, nil)
	if err != nil {
		log.Printf("%s: Failed to delete admin command message %d: %v",
			utils.GetCurrentTypeName(), adminMsg.MessageId, err)
	}

	// Send success message to admin DM
	_, err = h.messageSenderService.SendWithReturnMessage(h.getAdminUserID(adminMsg), "✅ Сообщение успешно удалено из базы данных.", nil)
	if err != nil {
		log.Printf("%s: Failed to send success message to admin: %v", utils.GetCurrentTypeName(), err)
	}

	return nil
}

func (h *AdminMessageControlHandler) getAdminUserID(adminMsg *gotgbot.Message) int64 {
	adminUserID := adminMsg.From.Id
	if adminMsg.From.IsBot && adminMsg.From.Username == "GroupAnonymousBot" {
		adminUserID = h.config.AdminUserID
	}
	return adminUserID
}
