package testhandlers

import (
	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories" // User model is also in here
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	tryGenerateCoffeePairsStateAwaitConfirmation = "try_generate_coffee_pairs_state_await_confirmation"

	// UserStore keys
	tryGenerateCoffeePairsCtxDataKeyPreviousMessageID = "try_generate_coffee_pairs_ctx_data_previous_message_id"
	tryGenerateCoffeePairsCtxDataKeyPreviousChatID    = "try_generate_coffee_pairs_ctx_data_previous_chat_id"

	// Menu headers
	tryGenerateCoffeePairsMenuHeader = "Генерация пар для Random Coffee"
)

type tryGenerateCoffeePairsHandler struct {
	config          *config.Config
	permissions     *services.PermissionsService
	sender          *services.MessageSenderService
	pollRepo        *repositories.RandomCoffeePollRepository
	participantRepo *repositories.RandomCoffeeParticipantRepository
	profileRepo     *repositories.ProfileRepository
	userStore       *utils.UserDataStore
}

func NewTryGenerateCoffeePairsHandler(
	config *config.Config,
	permissions *services.PermissionsService,
	sender *services.MessageSenderService,
	pollRepo *repositories.RandomCoffeePollRepository,
	participantRepo *repositories.RandomCoffeeParticipantRepository,
	profileRepo *repositories.ProfileRepository,
) ext.Handler {
	h := &tryGenerateCoffeePairsHandler{
		config:          config,
		permissions:     permissions,
		sender:          sender,
		pollRepo:        pollRepo,
		participantRepo: participantRepo,
		profileRepo:     profileRepo,
		userStore:       utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TryGenerateCoffeePairsCommand, h.handleCommand),
		},
		map[string][]ext.Handler{
			tryGenerateCoffeePairsStateAwaitConfirmation: {
				handlers.NewCallback(callbackquery.Equal(constants.TryGenerateCoffeePairsConfirmCallback), h.handleConfirmCallback),
				handlers.NewCallback(callbackquery.Equal(constants.TryGenerateCoffeePairsBackCallback), h.handleBackCallback),
				handlers.NewCallback(callbackquery.Equal(constants.TryGenerateCoffeePairsCancelCallback), h.handleCancelCallback),
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

// Entry point for the /coffeeGeneratePairs command
func (h *tryGenerateCoffeePairsHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissions.CheckAdminAndPrivateChat(msg, constants.TryGenerateCoffeePairsCommand) {
		log.Printf("TryGenerateCoffeePairsHandler: User %d (%s) tried to use /%s without admin permissions.",
			ctx.EffectiveUser.Id, ctx.EffectiveUser.Username, constants.TryGenerateCoffeePairsCommand)
		return handlers.EndConversation()
	}

	return h.showConfirmationMenu(b, ctx.EffectiveMessage, ctx.EffectiveUser.Id)
}

// Shows the confirmation menu for generating coffee pairs
func (h *tryGenerateCoffeePairsHandler) showConfirmationMenu(b *gotgbot.Bot, msg *gotgbot.Message, userId int64) error {
	h.RemovePreviousMessage(b, &userId)

	// Get latest poll info to show in confirmation
	latestPoll, err := h.pollRepo.GetLatestPoll()
	if err != nil {
		h.sender.Reply(msg, "Ошибка при получении информации об опросе.", nil)
		return handlers.EndConversation()
	}
	if latestPoll == nil {
		h.sender.Reply(msg, "Опрос для рандом кофе не найден.", nil)
		return handlers.EndConversation()
	}

	participants, err := h.participantRepo.GetParticipatingUsers(latestPoll.ID)
	if err != nil {
		h.sender.Reply(msg, "Ошибка при получении списка участников.", nil)
		return handlers.EndConversation()
	}

	editedMsg, err := h.sender.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", tryGenerateCoffeePairsMenuHeader)+
			"\n\n⚠️ ЭТА КОМАНДА НУЖНА ДЛЯ ТЕСТИРОВАНИЯ ФУНКЦИОНАЛА!"+
			"\n\nВы уверены, что хотите сгенерировать пары для текущего опроса?"+
			fmt.Sprintf("\n\n📊 Опрос: неделя %s", latestPoll.WeekStartDate.Format("2006-01-02"))+
			fmt.Sprintf("\n👥 Участников: %d", len(participants))+
			"\n\n⚠️ Пары будут отправлены в сообщество.",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ConfirmAndCancelButton(
				constants.TryGenerateCoffeePairsConfirmCallback,
				constants.TryGenerateCoffeePairsCancelCallback,
			),
		})

	if err != nil {
		return fmt.Errorf("TryGenerateCoffeePairsHandler: failed to send message in showConfirmationMenu: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(tryGenerateCoffeePairsStateAwaitConfirmation)
}

func (h *tryGenerateCoffeePairsHandler) handleConfirmCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	h.RemovePreviousMessage(b, &userId)

	// Show processing message
	editedMsg, err := h.sender.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", tryGenerateCoffeePairsMenuHeader)+
			"\n\n⏳ Генерация пар...",
		nil)
	h.SavePreviousMessageInfo(userId, editedMsg)
	if err != nil {
		return fmt.Errorf("TryGenerateCoffeePairsHandler: failed to send processing message: %w", err)
	}

	// Execute the pairs generation logic
	err = h.generateAndSendPairs()
	if err != nil {
		h.RemovePreviousMessage(b, &userId)

		// Send new error message with buttons
		editedMsg, sendErr := h.sender.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", tryGenerateCoffeePairsMenuHeader)+
				"\n\n❌ Ошибка при генерации пар:"+
				fmt.Sprintf("\n<code>%s</code>", err.Error())+
				"\n\nВернуться к подтверждению?",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.BackAndCancelButton(
					constants.TryGenerateCoffeePairsBackCallback,
					constants.TryGenerateCoffeePairsCancelCallback,
				),
			})
		if sendErr != nil {
			return fmt.Errorf("TryGenerateCoffeePairsHandler: failed to send error message: %w", sendErr)
		}

		h.SavePreviousMessageInfo(userId, editedMsg)
		return nil // Stay in the same state to allow retry
	}

	h.RemovePreviousMessage(b, &userId)

	// Send success message
	err = h.sender.SendHtml(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", tryGenerateCoffeePairsMenuHeader)+
			"\n\n✅ Пары успешно сгенерированы и отправлены в супергруппу!",
		nil)

	if err != nil {
		return fmt.Errorf("TryGenerateCoffeePairsHandler: failed to send success message: %w", err)
	}

	h.userStore.Clear(userId)
	return handlers.EndConversation()
}

// Generate pairs and send them to the supergroup
func (h *tryGenerateCoffeePairsHandler) generateAndSendPairs() error {
	latestPoll, err := h.pollRepo.GetLatestPoll()
	if err != nil {
		return fmt.Errorf("error getting latest poll: %w", err)
	}
	if latestPoll == nil {
		return fmt.Errorf("опрос для рандом кофе не найден")
	}

	participants, err := h.participantRepo.GetParticipatingUsers(latestPoll.ID)
	if err != nil {
		return fmt.Errorf("error getting participants for poll ID %d: %w", latestPoll.ID, err)
	}

	if len(participants) < 2 {
		return fmt.Errorf("недостаточно участников для создания пар (нужно минимум 2, зарегистрировалось %d)", len(participants))
	}

	// Random Pairing Logic
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(participants), func(i, j int) {
		participants[i], participants[j] = participants[j], participants[i]
	})

	var pairsText []string
	var unpairedUserText string

	for i := 0; i < len(participants); i += 2 {
		user1 := participants[i]
		user1Display := h.formatUserDisplay(&user1)

		if i+1 < len(participants) {
			user2 := participants[i+1]
			user2Display := h.formatUserDisplay(&user2)
			pairsText = append(pairsText, fmt.Sprintf("%s x %s", user1Display, user2Display))
		} else {
			unpairedUserText = user1Display
		}
	}

	var messageBuilder strings.Builder
	messageBuilder.WriteString(fmt.Sprintf("☕️ Пары для рандом кофе (неделя %s):\n\n", latestPoll.WeekStartDate.Format("Mon, Jan 2")))
	for _, pair := range pairsText {
		messageBuilder.WriteString(fmt.Sprintf("➪ %s\n", pair))
	}
	if unpairedUserText != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n😔 %s ищет кофе-компаньона на эту неделю!\n", unpairedUserText))
	}
	messageBuilder.WriteString("\n🗓 День, время и формат встречи вы выбираете сами. Просто напиши партнеру в личку, когда и в каком формате тебе удобно встретиться.")

	// Send the pairing message
	chatID := utils.ChatIdToFullChatId(h.config.SuperGroupChatID)
	opts := &gotgbot.SendMessageOpts{
		MessageThreadId: int64(h.config.RandomCoffeeTopicID),
	}

	err = h.sender.SendHtml(chatID, messageBuilder.String(), opts)
	if err != nil {
		return fmt.Errorf("error sending pairing message to chat %d: %w", chatID, err)
	}

	log.Printf("TryGenerateCoffeePairsHandler: Successfully sent pairings for poll ID %d to chat %d.", latestPoll.ID, h.config.SuperGroupChatID)
	return nil
}

func (h *tryGenerateCoffeePairsHandler) handleBackCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	return h.showConfirmationMenu(b, msg, userId)
}

func (h *tryGenerateCoffeePairsHandler) handleCancelCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return h.handleCancel(b, ctx)
}

func (h *tryGenerateCoffeePairsHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	h.RemovePreviousMessage(b, &userId)
	err := h.sender.Send(
		msg.Chat.Id,
		"Генерация пар для Random Coffee отменена.",
		nil)
	if err != nil {
		return fmt.Errorf("TryGenerateCoffeePairsHandler: failed to send cancel message: %w", err)
	}
	h.userStore.Clear(userId)

	return handlers.EndConversation()
}

func (h *tryGenerateCoffeePairsHandler) RemovePreviousMessage(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			tryGenerateCoffeePairsCtxDataKeyPreviousMessageID,
			tryGenerateCoffeePairsCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	b.DeleteMessage(chatID, messageID, nil)
}

func (h *tryGenerateCoffeePairsHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(
		userID,
		sentMsg.MessageId,
		sentMsg.Chat.Id,
		tryGenerateCoffeePairsCtxDataKeyPreviousMessageID,
		tryGenerateCoffeePairsCtxDataKeyPreviousChatID,
	)
}

// formatUserDisplay formats user display based on whether they have a published profile
func (h *tryGenerateCoffeePairsHandler) formatUserDisplay(user *repositories.User) string {
	// Get user profile to check if it's published
	profile, err := h.profileRepo.GetOrCreate(user.ID)
	if err != nil {
		log.Printf("TryGenerateCoffeePairsHandler: Error getting profile for user %d: %v", user.ID, err)
		// Fallback to username only
		if user.TgUsername != "" {
			return fmt.Sprintf("@%s", user.TgUsername)
		}
		return user.Firstname
	}

	// Check if profile is published (has published_message_id)
	hasPublishedProfile := profile.PublishedMessageID.Valid && profile.PublishedMessageID.Int64 > 0

	if hasPublishedProfile {
		// Show full name with username for published profiles, wrap name in link
		fullName := user.Firstname
		if user.Lastname != "" {
			fullName += " " + user.Lastname
		}

		// Get link to the published profile post
		profileLink := utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64)
		linkedName := fmt.Sprintf("<a href=\"%s\">%s</a>", profileLink, fullName)

		if user.TgUsername != "" {
			return fmt.Sprintf("%s (@%s)", linkedName, user.TgUsername)
		}
		return linkedName
	} else {
		// Show only username for unpublished profiles
		if user.TgUsername != "" {
			return fmt.Sprintf("@%s", user.TgUsername)
		}
		return user.Firstname
	}
}
