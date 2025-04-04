package adminhandlers

import (
	"fmt"
	"log"
	"strings"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	codeHandlerStateWaitForCode = "code_handler_wait_for_code"
)

type codeHandler struct {
	config               *config.Config
	messageSenderService services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewCodeHandler(
	config *config.Config,
	messageSenderService services.MessageSenderService,
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
	if !h.permissionsService.CheckAdminAndPrivateChat(b, ctx, constants.ShowTopicsCommand) {
		return handlers.EndConversation()
	}

	// Ask user to enter the code
	h.messageSenderService.Reply(b, msg, fmt.Sprintf("Пожалуйста, введите код или используйте /%s для отмены:", constants.CancelCommand), nil)

	return handlers.NextConversationState(codeHandlerStateWaitForCode)
}

// processCode handles the code input from the user
func (h *codeHandler) processCode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract code from message
	revertedCode := strings.TrimSpace(msg.Text)
	if revertedCode == "" {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf("Код не может быть пустым. Пожалуйста, введите код или используйте /%s для отмены:", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	code := reverseString(revertedCode)

	// Store the code in memory
	clients.TgSetVerificationCode(code)
	log.Print("Code stored")
	err := clients.TgKeepSessionAlive() // Refresh session
	if err == nil {
		h.messageSenderService.Reply(b, msg, "Код принят", nil)
	} else {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при сохранении кода", nil)
		log.Printf("CodeHandler: Error during code storage: %v", err)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCancel handles the /cancel command
func (h *codeHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	h.messageSenderService.Reply(b, msg, "Операция ввода кода отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
