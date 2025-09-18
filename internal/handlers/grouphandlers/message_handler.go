package grouphandlers

import (
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/services/grouphandlersservices"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

type MessageHandler struct {
	messageSenderService            *services.MessageSenderService
	cleanClosedThreadsService       *grouphandlersservices.CleanClosedThreadsService
	repliesFromClosedThreadsService *grouphandlersservices.RepliesFromClosedThreadsService
	deleteJoinLeftMessagesService   *grouphandlersservices.DeleteJoinLeftMessagesService
	saveTopicService                *grouphandlersservices.SaveTopicService
	adminSaveMessageService         *grouphandlersservices.AdminSaveMessageService
	saveMessageService              *grouphandlersservices.SaveMessageService
}

func NewMessageHandler(
	messageSenderService *services.MessageSenderService,
	cleanClosedThreadsService *grouphandlersservices.CleanClosedThreadsService,
	repliesFromClosedThreadsService *grouphandlersservices.RepliesFromClosedThreadsService,
	deleteJoinLeftMessagesService *grouphandlersservices.DeleteJoinLeftMessagesService,
	saveTopicService *grouphandlersservices.SaveTopicService,
	adminSaveMessageService *grouphandlersservices.AdminSaveMessageService,
	saveMessageService *grouphandlersservices.SaveMessageService,
) ext.Handler {
	h := &MessageHandler{
		messageSenderService:            messageSenderService,
		cleanClosedThreadsService:       cleanClosedThreadsService,
		repliesFromClosedThreadsService: repliesFromClosedThreadsService,
		deleteJoinLeftMessagesService:   deleteJoinLeftMessagesService,
		saveTopicService:                saveTopicService,
		adminSaveMessageService:         adminSaveMessageService,
		saveMessageService:              saveMessageService,
	}

	return handlers.NewMessage(message.All, h.handle).SetAllowEdited(true)
}

func (h *MessageHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsMessageFromSuperGroupChat(ctx.EffectiveMessage.Chat) {
		return nil
	}

	msg := ctx.EffectiveMessage

	// Delete join left messages and finish processing
	if h.deleteJoinLeftMessagesService.IsMessageShouldBeDeleted(msg) {
		return h.deleteJoinLeftMessagesService.DeleteJoinLeftMessages(msg, b)
	}

	// Clean closed threads and finish processing
	if h.cleanClosedThreadsService.IsTopicShouldBeCleaned(msg, b) {
		return h.cleanClosedThreadsService.CleanClosedThreads(msg, b)
	}

	// Process replies from closed threads and finish processing
	if h.repliesFromClosedThreadsService.IsReplyShouldBeForwarded(msg, b) {
		return h.repliesFromClosedThreadsService.RepliesFromClosedThreads(msg, b, ctx)
	}

	// Save or update topic and finish processing
	if h.saveTopicService.IsTopicShouldBeSavedOrUpdated(msg) {
		return h.saveTopicService.SaveOrUpdateTopic(msg)
	}

	// Save or delete message in Content and Tools topics
	// by admin command, than finish processing
	if h.adminSaveMessageService.IsMessageShouldBeSavedOrUpdated(msg) {
		return h.adminSaveMessageService.SaveOrUpdateMessage(msg)
	}

	// Save or update or delete message in DB, than finish processing
	if h.saveMessageService.IsMessageShouldBeSavedOrUpdated(msg) {
		return h.saveMessageService.SaveOrUpdateMessage(ctx)
	}

	return nil
}
