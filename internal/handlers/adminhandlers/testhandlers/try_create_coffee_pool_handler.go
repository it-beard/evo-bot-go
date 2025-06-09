package testhandlers

import (
	"context"
	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	tryCreateCoffeePoolStateAwaitConfirmation = "try_create_coffee_pool_state_await_confirmation"

	// UserStore keys
	tryCreateCoffeePoolCtxDataKeyPreviousMessageID = "try_create_coffee_pool_ctx_data_previous_message_id"
	tryCreateCoffeePoolCtxDataKeyPreviousChatID    = "try_create_coffee_pool_ctx_data_previous_chat_id"

	// Menu headers
	tryCreateCoffeePoolMenuHeader = "Запуск опроса по кофейным встречам"
)

type tryCreateCoffeePoolHandler struct {
	config               *config.Config
	messageSenderService *services.MessageSenderService
	permissionsService   *services.PermissionsService
	randomCoffeeService  *services.RandomCoffeeService
	userStore            *utils.UserDataStore
}

func NewTryCreateCoffeePoolHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
	randomCoffeeService *services.RandomCoffeeService,
) ext.Handler {
	h := &tryCreateCoffeePoolHandler{
		config:               config,
		messageSenderService: messageSenderService,
		permissionsService:   permissionsService,
		randomCoffeeService:  randomCoffeeService,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TryCreateCoffeePoolCommand, h.handleCommand),
		},
		map[string][]ext.Handler{
			tryCreateCoffeePoolStateAwaitConfirmation: {
				handlers.NewCallback(callbackquery.Equal(constants.TryCreateCoffeePoolConfirmCallback), h.handleConfirmCallback),
				handlers.NewCallback(callbackquery.Equal(constants.TryCreateCoffeePoolCancelCallback), h.handleCancelCallback),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
			Fallbacks: []ext.Handler{
				handlers.NewMessage(message.Text, func(b *gotgbot.Bot, ctx *ext.Context) error {
					// Delete the message that not matched any state
					b.DeleteMessage(ctx.EffectiveMessage.Chat.Id, ctx.EffectiveMessage.MessageId, nil)
					return nil
				}),
			},
		},
	)
}

// Entry point for the /coofeeRestart command
func (h *tryCreateCoffeePoolHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.TryCreateCoffeePoolCommand) {
		log.Printf("%s: User %d (%s) tried to use /%s without admin permissions.",
			utils.GetCurrentTypeName(),
			ctx.EffectiveUser.Id,
			ctx.EffectiveUser.Username,
			constants.TryCreateCoffeePoolCommand,
		)
		return handlers.EndConversation()
	}

	return h.showConfirmationMenu(b, ctx.EffectiveMessage, ctx.EffectiveUser.Id)
}

// Shows the confirmation menu for starting a new coffee poll
func (h *tryCreateCoffeePoolHandler) showConfirmationMenu(b *gotgbot.Bot, msg *gotgbot.Message, userId int64) error {
	h.RemovePreviousMessage(b, &userId)

	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", tryCreateCoffeePoolMenuHeader)+
			"\n\n⚠️ ЭТА КОМАНДА НУЖНА ДЛЯ ТЕСТИРОВАНИЯ ФУНКЦИОНАЛА!"+
			"\n\nВы уверены, что хотите запустить новый опрос по кофейным встречам?"+
			fmt.Sprintf("\n\nОпрос будет отправлен в топик \"Random Coffee\" (ID: %d).", h.config.RandomCoffeeTopicID),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ConfirmAndCancelButton(
				constants.TryCreateCoffeePoolConfirmCallback,
				constants.TryCreateCoffeePoolCancelCallback,
			),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in showConfirmationMenu: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(tryCreateCoffeePoolStateAwaitConfirmation)
}

// Handle the "Да" (Confirm) button click
func (h *tryCreateCoffeePoolHandler) handleConfirmCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	h.RemovePreviousMessage(b, &userId)

	// Show processing message
	processingMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", tryCreateCoffeePoolMenuHeader)+
			"\n\n⏳ Создание опроса...",
		nil)

	if err != nil {
		return fmt.Errorf("%s: failed to send processing message: %w", utils.GetCurrentTypeName(), err)
	}

	// Create the poll using the service
	err = h.randomCoffeeService.SendPoll(context.Background())
	if err != nil {
		// Update message with error
		_, _, editErr := b.EditMessageText(
			fmt.Sprintf("<b>%s</b>", tryCreateCoffeePoolMenuHeader)+
				"\n\n❌ Ошибка при создании опроса:"+
				fmt.Sprintf("\n<code>%s</code>", err.Error()),
			&gotgbot.EditMessageTextOpts{
				ChatId:    msg.Chat.Id,
				MessageId: processingMsg.MessageId,
				ParseMode: "HTML",
			})
		if editErr != nil {
			return fmt.Errorf("%s: failed to edit error message: %w", utils.GetCurrentTypeName(), editErr)
		}
		return fmt.Errorf("%s: failed to create coffee poll: %w", utils.GetCurrentTypeName(), err)
	}

	// Update message with success
	_, _, err = b.EditMessageText(
		fmt.Sprintf("<b>%s</b>", tryCreateCoffeePoolMenuHeader)+
			"\n\n✅ Опрос по кофейным встречам успешно создан и отправлен в группу!",
		&gotgbot.EditMessageTextOpts{
			ChatId:    msg.Chat.Id,
			MessageId: processingMsg.MessageId,
			ParseMode: "HTML",
		})

	if err != nil {
		return fmt.Errorf("%s: failed to update success message: %w", utils.GetCurrentTypeName(), err)
	}

	h.userStore.Clear(userId)
	return handlers.EndConversation()
}

// Handle the "Нет" (Cancel) button click
func (h *tryCreateCoffeePoolHandler) handleCancelCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return h.handleCancel(b, ctx)
}

func (h *tryCreateCoffeePoolHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	h.RemovePreviousMessage(b, &userId)
	err := h.messageSenderService.Send(
		msg.Chat.Id,
		"Создание опроса по кофейным встречам отменено.",
		nil)
	if err != nil {
		return fmt.Errorf("%s: failed to send cancel message: %w", utils.GetCurrentTypeName(), err)
	}
	h.userStore.Clear(userId)

	return handlers.EndConversation()
}

func (h *tryCreateCoffeePoolHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			tryCreateCoffeePoolCtxDataKeyPreviousMessageID,
			tryCreateCoffeePoolCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *tryCreateCoffeePoolHandler) RemovePreviousMessage(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			tryCreateCoffeePoolCtxDataKeyPreviousMessageID,
			tryCreateCoffeePoolCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	b.DeleteMessage(chatID, messageID, nil)
}

func (h *tryCreateCoffeePoolHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(
		userID,
		sentMsg.MessageId,
		sentMsg.Chat.Id,
		tryCreateCoffeePoolCtxDataKeyPreviousMessageID,
		tryCreateCoffeePoolCtxDataKeyPreviousChatID,
	)
}
