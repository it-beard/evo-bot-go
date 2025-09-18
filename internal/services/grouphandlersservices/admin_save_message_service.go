package grouphandlersservices

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"log"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type AdminSaveMessageService struct {
	groupMessageRepository   *repositories.GroupMessageRepository
	bot                      *gotgbot.Bot
	config                   *config.Config
	saveUpdateMessageService *SaveUpdateMessageService
	messageSenderService     *services.MessageSenderService
}

func NewAdminSaveMessageService(
	groupMessageRepository *repositories.GroupMessageRepository,
	bot *gotgbot.Bot,
	config *config.Config,
	saveUpdateMessageService *SaveUpdateMessageService,
	messageSenderService *services.MessageSenderService,
) *AdminSaveMessageService {
	return &AdminSaveMessageService{
		groupMessageRepository:   groupMessageRepository,
		bot:                      bot,
		config:                   config,
		saveUpdateMessageService: saveUpdateMessageService,
		messageSenderService:     messageSenderService,
	}
}

func (s *AdminSaveMessageService) SaveOrUpdateMessage(msg *gotgbot.Message) error {
	command := strings.ToLower(strings.TrimSpace(msg.Text))
	repliedMessage := msg.ReplyToMessage

	switch command {
	case constants.AdminSaveMessage_ReplyUpdateMesageCommand:
		return s.handleUpdateCommand(msg, repliedMessage)
	case constants.AdminSaveMessage_ReplyDeleteMessageCommand:
		return s.handleDeleteCommand(msg, repliedMessage)
	default:
		return nil // Should not reach here due to check() method
	}
}

func (s *AdminSaveMessageService) IsMessageShouldBeSavedOrUpdated(msg *gotgbot.Message) bool {
	// Must be a reply to another message
	if msg.ReplyToMessage == nil {
		return false
	}

	// Must be in content or tool topic
	if msg.MessageThreadId != int64(s.config.ContentTopicID) &&
		msg.MessageThreadId != int64(s.config.ToolTopicID) {
		return false
	}

	// Must be from an admin or GroupAnonymousBot
	if !utils.IsUserAdminOrCreator(s.bot, msg.From.Id, s.config) &&
		(msg.From.IsBot && msg.From.Username != "GroupAnonymousBot") {
		return false
	}

	// Must be "update" or "delete" command
	return msg.Text == constants.AdminSaveMessage_ReplyUpdateMesageCommand ||
		msg.Text == constants.AdminSaveMessage_ReplyDeleteMessageCommand
}

func (h *AdminSaveMessageService) handleUpdateCommand(adminMsg *gotgbot.Message, repliedMessage *gotgbot.Message) error {
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

	// Delete the admin's command message after 5 seconds in a goroutine to avoid blocking
	go func() {
		time.Sleep(10 * time.Second)
		_, err := adminMsg.Delete(h.bot, nil)
		if err != nil {
			log.Printf("%s: Failed to delete admin command message %d: %v",
				utils.GetCurrentTypeName(), adminMsg.MessageId, err)
		}
	}()

	// Send success message to admin DM
	_, err = h.messageSenderService.SendWithReturnMessage(h.getAdminUserID(adminMsg), "✅ Сообщение успешно добавлено/обновлено в базе данных.", nil)
	if err != nil {
		log.Printf("%s: Failed to reply to admin: %v", utils.GetCurrentTypeName(), err)
	}

	return nil
}

func (h *AdminSaveMessageService) handleDeleteCommand(adminMsg *gotgbot.Message, repliedMessage *gotgbot.Message) error {

	// First, try to delete the replied message from Telegram and database
	err := h.saveUpdateMessageService.Delete(repliedMessage)
	if err != nil {
		log.Printf("%s: Failed to delete replied message %d: %v",
			utils.GetCurrentTypeName(), repliedMessage.MessageId, err)
	}

	// Delete the admin's command message after 5 seconds in a goroutine to avoid blocking
	go func() {
		time.Sleep(10 * time.Second)
		_, err := adminMsg.Delete(h.bot, nil)
		if err != nil {
			log.Printf("%s: Failed to delete admin command message %d: %v",
				utils.GetCurrentTypeName(), adminMsg.MessageId, err)
		}
	}()

	// Send success message to admin DM
	_, err = h.messageSenderService.SendWithReturnMessage(h.getAdminUserID(adminMsg), "✅ Сообщение успешно удалено из базы данных.", nil)
	if err != nil {
		log.Printf("%s: Failed to send success message to admin: %v", utils.GetCurrentTypeName(), err)
	}

	return nil
}

func (h *AdminSaveMessageService) getAdminUserID(adminMsg *gotgbot.Message) int64 {
	adminUserID := adminMsg.From.Id
	if adminMsg.From.IsBot && adminMsg.From.Username == "GroupAnonymousBot" {
		adminUserID = h.config.AdminUserID
	}
	return adminUserID
}
