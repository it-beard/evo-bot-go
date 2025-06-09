package adminhandlers

import (
	"fmt"
	"log"
	"strings"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states names
	codeHandlerStateWaitForCode = "code_handler_state_wait_for_code"

	// Context data keys
	codeHandlerCtxDataKeyPreviousMessageID = "code_handler_ctx_data_previous_message_id"
	codeHandlerCtxDataKeyPreviousChatID    = "code_handler_ctx_data_previous_chat_id"

	// Callback data
	codeHandlerCallbackConfirmCancel = "code_handler_callback_confirm_cancel"
)

type codeHandler struct {
	config               *config.Config
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewCodeHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &codeHandler{
		config:               config,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.CodeCommand, h.startCodeConversation),
		},
		map[string][]ext.Handler{
			codeHandlerStateWaitForCode: {
				handlers.NewMessage(message.All, h.processCode),
				handlers.NewCallback(callbackquery.Equal(codeHandlerCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// startCodeConversation initiates the code input conversation
func (h *codeHandler) startCodeConversation(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.ShowTopicsCommand) {
		log.Printf("%s: User %d (%s) tried to use /%s without admin permissions.",
			utils.GetCurrentTypeName(),
			ctx.EffectiveUser.Id,
			ctx.EffectiveUser.Username,
			constants.ShowTopicsCommand,
		)
		return handlers.EndConversation()
	}

	// Ask user to enter the code
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(
		msg,
		fmt.Sprintf("Пожалуйста, введите код:"),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(codeHandlerCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(codeHandlerStateWaitForCode)
}

// processCode handles the code input from the user
func (h *codeHandler) processCode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract code from message
	revertedCode := strings.TrimSpace(msg.Text)
	if revertedCode == "" {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Код не может быть пустым. Пожалуйста, введите код или используйте кнопку для отмены."),
			nil,
		)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	code := reverseString(revertedCode)

	// Store the code in memory
	clients.TgSetVerificationCode(code)
	log.Print("Code stored")
	err := clients.TgKeepSessionAlive() // Refresh session
	if err == nil {
		h.messageSenderService.Reply(msg, "Код принят", nil)
	} else {
		h.messageSenderService.Reply(msg, "Произошла ошибка при сохранении кода", nil)
		log.Printf("%s: Error during code storage: %v", utils.GetCurrentTypeName(), err)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *codeHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// handleCancel handles the /cancel command
func (h *codeHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	h.messageSenderService.Reply(msg, "Операция ввода кода отменена.", nil)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *codeHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			codeHandlerCtxDataKeyPreviousMessageID,
			codeHandlerCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *codeHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		codeHandlerCtxDataKeyPreviousMessageID, codeHandlerCtxDataKeyPreviousChatID)
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
