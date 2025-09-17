package privatehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
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
	introStateProcessQuery = "intro_state_process_query"

	// UserStore keys
	introCtxDataKeyProcessing        = "intro_ctx_data_processing"
	introCtxDataKeyCancelFunc        = "intro_ctx_data_cancel_func"
	introCtxDataKeyPreviousMessageID = "intro_ctx_data_previous_message_id"
	introCtxDataKeyPreviousChatID    = "intro_ctx_data_previous_chat_id"

	// Callback data
	introCallbackConfirmCancel = "intro_callback_confirm_cancel"
)

type introHandler struct {
	config                      *config.Config
	openaiClient                *clients.OpenAiClient
	promptingTemplateRepository *repositories.PromptingTemplateRepository
	profileRepository           *repositories.ProfileRepository
	messageSenderService        *services.MessageSenderService
	userStore                   *utils.UserDataStore
	permissionsService          *services.PermissionsService
}

func NewIntroHandler(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService *services.MessageSenderService,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
	profileRepository *repositories.ProfileRepository,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &introHandler{
		config:                      config,
		openaiClient:                openaiClient,
		promptingTemplateRepository: promptingTemplateRepository,
		profileRepository:           profileRepository,
		messageSenderService:        messageSenderService,
		userStore:                   utils.NewUserDataStore(),
		permissionsService:          permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.IntroCommand, h.startIntroSearch),
		},
		map[string][]ext.Handler{
			introStateProcessQuery: {
				handlers.NewMessage(message.All, h.processIntroSearch),
				handlers.NewCallback(callbackquery.Equal(introCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{
				handlers.NewCommand(constants.CancelCommand, h.handleCancel),
				handlers.NewCallback(callbackquery.Equal(introCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
	)
}

// startIntroSearch is the entry point handler for the intro search conversation
func (h *introHandler) startIntroSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.IntroCommand) {
		return handlers.EndConversation()
	}

	// Ask user to enter search query
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(
		msg,
		fmt.Sprintf("Введи поисковый запрос по участникам клуба или нажми /%s для получения общей информации:", constants.CancelCommand),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(introCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(introStateProcessQuery)
}

// processIntroSearch handles the actual intro search
func (h *introHandler) processIntroSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if we're already processing a request for this user
	if isProcessing, ok := h.userStore.Get(ctx.EffectiveUser.Id, introCtxDataKeyProcessing); ok && isProcessing.(bool) {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, дождись окончания обработки предыдущего запроса, или используй /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get query from user message
	query := strings.TrimSpace(msg.Text)

	// Mark as processing
	h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyProcessing, true)

	// Create a cancellable context for this operation
	typingCtx, cancelTyping := context.WithCancel(context.Background())

	// Store cancel function in user store so it can be called from handleCancel
	h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyCancelFunc, cancelTyping)

	// Make sure we clean up the processing flag in all exit paths
	defer func() {
		h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyProcessing, false)
		h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyCancelFunc, nil)
	}()

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Inform user that processing has started
	var sentMsg *gotgbot.Message
	if query == "" {
		sentMsg, _ = h.messageSenderService.ReplyWithReturnMessage(msg, "Генерирую вводную информацию о клубе...", &gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(introCallbackConfirmCancel),
		})
	} else {
		sentMsg, _ = h.messageSenderService.ReplyWithReturnMessage(msg, fmt.Sprintf("Ищу информацию по запросу: \"%s\"...", query), &gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(introCallbackConfirmCancel),
		})
	}
	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get profile data from repository
	profiles, err := h.prepareProfileData()
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении данных профилей для обработки.", nil)
		log.Printf("%s: Error during profile data preparation: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	// Get the prompt template from the database
	templateText, err := h.promptingTemplateRepository.Get(prompts.GetIntroPromptKey, prompts.GetIntroPromptDefaultValue)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении шаблона для вводной информации.", nil)
		log.Printf("%s: Error during template retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.IntroTopicID)

	prompt := fmt.Sprintf(
		templateText,
		topicLink,
		utils.EscapeMarkdown(string(profiles)),
		utils.EscapeMarkdown(query),
	)

	// Save the prompt into a temporary file for logging purposes.
	err = os.WriteFile("last-prompt-log.txt", []byte(prompt), 0644)
	if err != nil {
		log.Printf("%s: Error writing prompt to file: %v", utils.GetCurrentTypeName(), err)
	}

	// Start periodic typing action every 5 seconds while waiting for the OpenAI response.
	defer cancelTyping() // ensure cancellation if function exits early

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.messageSenderService.SendTypingAction(msg.Chat.Id)
			case <-typingCtx.Done():
				return
			}
		}
	}()

	// Get completion from OpenAI using the new context.
	responseOpenAi, err := h.openaiClient.GetCompletion(typingCtx, prompt)
	// Check if context was cancelled
	if typingCtx.Err() != nil {
		log.Printf("%s: Request was cancelled", utils.GetCurrentTypeName())
		return handlers.EndConversation()
	}

	// Continue only if no errors
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении ответа от OpenAI.", nil)
		log.Printf("%s: Error during OpenAI response retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	err = h.messageSenderService.ReplyHtml(msg, responseOpenAi, nil)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при отправке ответа.", nil)
		log.Printf("%s: Error during message sending: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *introHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// handleCancel handles the /cancel command
func (h *introHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, introCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(msg, "Операция поиска вводной информации отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(msg, "Операция поиска вводной информации отменена.", nil)
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *introHandler) prepareProfileData() ([]byte, error) {
	type ProfileData struct {
		ID        int    `json:"id"`
		Firstname string `json:"firstname"`
		Lastname  string `json:"lastname"`
		Username  string `json:"username"`
		Bio       string `json:"bio"`
		MessageId *int64 `json:"message_id"`
	}

	// Get all profiles with users from the repository
	profilesWithUsers, err := h.profileRepository.GetAllWithUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to get profiles from repository: %w", err)
	}

	profileData := make([]ProfileData, 0, len(profilesWithUsers))
	for _, pwu := range profilesWithUsers {
		// Only include profiles with non-empty bios
		if pwu.Profile.Bio != "" {
			profileData = append(profileData, ProfileData{
				ID:        pwu.Profile.ID,
				Firstname: pwu.User.Firstname,
				Lastname:  pwu.User.Lastname,
				Username:  pwu.User.TgUsername,
				Bio:       pwu.Profile.Bio,
				MessageId: &pwu.Profile.PublishedMessageID.Int64,
			})
		}
	}

	if len(profileData) == 0 {
		return nil, fmt.Errorf("no profiles with bios found")
	}

	dataJSON, err := json.Marshal(profileData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal profile data to JSON: %w", err)
	}

	return dataJSON, nil
}

func (h *introHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			introCtxDataKeyPreviousMessageID,
			introCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *introHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		introCtxDataKeyPreviousMessageID, introCtxDataKeyPreviousChatID)
}
