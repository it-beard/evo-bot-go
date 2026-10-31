package testhandlers

import (
	"fmt"
	"log"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	tryLinkToLearnStateAwaitLanguage = "try_link_to_learn_state_await_language"

	// UserStore keys
	tryLinkToLearnCtxDataKeyPreviousMessageID = "try_link_to_learn_ctx_data_previous_message_id"
	tryLinkToLearnCtxDataKeyPreviousChatID    = "try_link_to_learn_ctx_data_previous_chat_id"

	// Direct link to the knowledge base Mini App; unlike the other links of the bot it stays on t.me
	tryLinkToLearnKnowledgeBaseURL = "https://t.me/the_evocoders_learn_bot/base?startapp=app"

	tryLinkToLearnChooseLanguageText = "На каком языке отправить сообщение со ссылкой на базу знаний?"
	tryLinkToLearnCanceledText       = "Отправка ссылки на базу знаний отменена."
)

// knowledgeBaseMessage is the knowledge base message in one language: its text and the label of the link button
type knowledgeBaseMessage struct {
	text       string
	buttonText string
}

// knowledgeBaseMessages maps each language button to the message sent for it
var knowledgeBaseMessages = map[string]knowledgeBaseMessage{
	constants.TryLinkToLearnRussianCallback:    {text: "База знаний Эволюции Кода ➡️", buttonText: "База знаний"},
	constants.TryLinkToLearnBelarusianCallback: {text: "База ведаў Эвалюцыі Кода ➡️", buttonText: "База ведаў"},
}

type tryLinkToLearnHandler struct {
	config               *config.Config
	messageSenderService *services.MessageSenderService
	permissionsService   *services.PermissionsService
	userStore            *utils.UserDataStore
}

func NewTryLinkToLearnHandler(
	cfg *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &tryLinkToLearnHandler{
		config:               cfg,
		messageSenderService: messageSenderService,
		permissionsService:   permissionsService,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TryLinkToLearnCommand, h.handleCommand),
		},
		map[string][]ext.Handler{
			tryLinkToLearnStateAwaitLanguage: {
				handlers.NewCallback(callbackquery.Equal(constants.TryLinkToLearnRussianCallback), h.handleLanguageCallback),
				handlers.NewCallback(callbackquery.Equal(constants.TryLinkToLearnBelarusianCallback), h.handleLanguageCallback),
				handlers.NewCallback(callbackquery.Equal(constants.TryLinkToLearnCancelCallback), h.handleCancelCallback),
				handlers.NewMessage(message.All, h.handleTextDuringSelection),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. handleCommand is the entry point of /tryLinkToLearn: it asks the admin for the language of the message
func (h *tryLinkToLearnHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.TryLinkToLearnCommand) {
		return handlers.EndConversation()
	}

	sentMsg, err := h.messageSenderService.SendWithReturnMessage(
		msg.Chat.Id,
		tryLinkToLearnChooseLanguageText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.LanguageSelectionButton(
				constants.TryLinkToLearnRussianCallback,
				constants.TryLinkToLearnBelarusianCallback,
				constants.TryLinkToLearnCancelCallback,
			),
		},
	)
	if err != nil {
		return fmt.Errorf("%s: failed to send language selection: %w", utils.GetCurrentTypeName(), err)
	}

	h.savePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(tryLinkToLearnStateAwaitLanguage)
}

// 2. handleLanguageCallback sends the knowledge base message in the chosen language
func (h *tryLinkToLearnHandler) handleLanguageCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	userID := ctx.EffectiveUser.Id

	// Answer the callback query to remove the loading state on the button
	_, _ = cb.Answer(b, nil)

	chosen, ok := knowledgeBaseMessages[cb.Data]
	if !ok {
		// Unreachable: this state only accepts the language buttons
		return fmt.Errorf("%s: unknown language button %q", utils.GetCurrentTypeName(), cb.Data)
	}

	err := h.messageSenderService.Send(
		ctx.EffectiveChat.Id,
		chosen.text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
					{
						Text: chosen.buttonText,
						Url:  tryLinkToLearnKnowledgeBaseURL,
					},
				}},
			},
		},
	)
	if err != nil {
		// The language buttons stay, so the admin can press one again or cancel
		return fmt.Errorf("%s: failed to send knowledge base link: %w", utils.GetCurrentTypeName(), err)
	}

	h.removePreviousMessage(b, userID)
	h.userStore.Clear(userID)
	return handlers.EndConversation()
}

// handleTextDuringSelection answers a text sent while the bot waits for a language button
func (h *tryLinkToLearnHandler) handleTextDuringSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	h.messageSenderService.Reply(ctx.EffectiveMessage, formatters.UseButtonsText, nil)
	return nil // Stay in the same state
}

// handleCancelCallback processes the cancel button click
func (h *tryLinkToLearnHandler) handleCancelCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	_, _ = ctx.Update.CallbackQuery.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// 3. handleCancel handles the /cancel command and the cancel button
func (h *tryLinkToLearnHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	userID := ctx.EffectiveUser.Id

	h.removePreviousMessage(b, userID)
	h.userStore.Clear(userID)

	if err := h.messageSenderService.Send(ctx.EffectiveChat.Id, tryLinkToLearnCanceledText, nil); err != nil {
		log.Printf("%s: failed to send cancel message: %v", utils.GetCurrentTypeName(), err)
	}

	return handlers.EndConversation()
}

// removePreviousMessage deletes the language selection message, if it is known
func (h *tryLinkToLearnHandler) removePreviousMessage(b *gotgbot.Bot, userID int64) {
	messageID, chatID := h.userStore.GetPreviousMessageInfo(
		userID,
		tryLinkToLearnCtxDataKeyPreviousMessageID,
		tryLinkToLearnCtxDataKeyPreviousChatID,
	)
	if chatID == 0 || messageID == 0 {
		return
	}

	if _, err := b.DeleteMessage(chatID, messageID, nil); err != nil {
		log.Printf("%s: failed to delete language selection message: %v", utils.GetCurrentTypeName(), err)
	}
}

func (h *tryLinkToLearnHandler) savePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(
		userID,
		sentMsg.MessageId,
		sentMsg.Chat.Id,
		tryLinkToLearnCtxDataKeyPreviousMessageID,
		tryLinkToLearnCtxDataKeyPreviousChatID,
	)
}
