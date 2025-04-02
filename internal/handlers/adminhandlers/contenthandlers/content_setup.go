package contenthandlers

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	stateAskContentName = "ask_content_name"
	stateAskContentType = "ask_content_type"

	// Context data keys
	ctxDataKeyContentName = "content_name"
	setupCancelCommand    = "cancel"
)

type contentSetupHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
	userStore         *setupUserDataStore
}

type setupUserDataStore struct {
	rwMux    sync.RWMutex
	userData map[int64]map[string]any
}

func NewContentSetupHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	store := &setupUserDataStore{
		userData: make(map[int64]map[string]any),
	}

	h := &contentSetupHandler{
		contentRepository: contentRepository,
		config:            config,
		userStore:         store,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentSetupCommand, h.startSetup),
		},
		map[string][]ext.Handler{
			stateAskContentName: {
				handlers.NewMessage(message.Text, h.handleContentName),
			},
			stateAskContentType: {
				handlers.NewMessage(message.Text, h.handleContentType),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(setupCancelCommand, h.handleCancel)},
		},
	)
}

// 1. startSetup is the entry point handler for the setup conversation
func (h *contentSetupHandler) startSetup(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		if _, err := msg.Reply(b, "Эта команда доступна только администраторам.", nil); err != nil {
			log.Printf("Failed to send admin-only message: %v", err)
		}
		log.Printf("User %d tried to use %s without admin rights", msg.From.Id, constants.ContentSetupCommand)
		return handlers.EndConversation()
	}

	// Check if the command is used in a private chat
	if msg.Chat.Type != constants.PrivateChatType {
		if _, err := msg.Reply(b, "Эта команда доступна только в личном чате.", nil); err != nil {
			log.Printf("Failed to send private-only message: %v", err)
		}
		return handlers.EndConversation()
	}

	if _, err := msg.Reply(b, fmt.Sprintf("Пожалуйста, введи название для нового контента или /%s для отмены:", setupCancelCommand), nil); err != nil {
		log.Printf("Failed to send name prompt: %v", err)
	}

	return handlers.NextConversationState(stateAskContentName)
}

// 2. handleContentName processes the content name input
func (h *contentSetupHandler) handleContentName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	contentName := strings.TrimSpace(msg.Text)

	if contentName == "" {
		if _, err := msg.Reply(b, fmt.Sprintf("Название не может быть пустым. Пожалуйста, введи название для контента или /%s для отмены:", setupCancelCommand), nil); err != nil {
			log.Printf("Failed to send empty name error: %v", err)
		}
		return nil // Stay in the same state
	}

	// Store the content name
	h.userStore.set(ctx.EffectiveUser.Id, ctxDataKeyContentName, contentName)

	// Ask for content type
	typeOptions := fmt.Sprintf("Выбери тип контента (введи число):\n1. %s\n2. %s\nИли /%s для отмены",
		constants.ContentTypeClubCall,
		constants.ContentTypeMeetup)

	if _, err := msg.Reply(b, typeOptions, nil); err != nil {
		log.Printf("Failed to send type options: %v", err)
	}

	return handlers.NextConversationState(stateAskContentType)
}

// 3. handleContentType processes the content type selection and creates the content
func (h *contentSetupHandler) handleContentType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	typeSelection := strings.TrimSpace(msg.Text)

	var contentType string
	switch typeSelection {
	case "1":
		contentType = constants.ContentTypeClubCall
	case "2":
		contentType = constants.ContentTypeMeetup
	default:
		if _, err := msg.Reply(b, fmt.Sprintf("Неверный выбор. Пожалуйста, введи 1 или 2, или /%s для отмены:", setupCancelCommand), nil); err != nil {
			log.Printf("Failed to send invalid type error: %v", err)
		}
		return nil // Stay in the same state
	}

	// Get the content name from user data store
	contentNameVal, ok := h.userStore.get(ctx.EffectiveUser.Id, ctxDataKeyContentName)
	if !ok {
		log.Printf("Error: content name not found in user data for user %d", ctx.EffectiveUser.Id)
		if _, err := msg.Reply(b, fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти название контента. Попробуй начать заново с /%s.", constants.ContentSetupCommand), nil); err != nil {
			log.Printf("Failed to send error message: %v", err)
		}
		return handlers.EndConversation()
	}

	contentName, ok := contentNameVal.(string)
	if !ok {
		log.Printf("Error: content name in user data is not a string: %T. Value: %v", contentNameVal, contentNameVal)
		if _, err := msg.Reply(b, fmt.Sprintf("Произошла внутренняя ошибка (неверный тип названия). Попробуй начать заново с /%s.", constants.ContentSetupCommand), nil); err != nil {
			log.Printf("Failed to send type error message: %v", err)
		}
		return handlers.EndConversation()
	}

	// Create content in the database
	id, err := h.contentRepository.CreateContent(contentName, contentType)
	if err != nil {
		log.Printf("Failed to create content: %v", err)
		if _, replyErr := msg.Reply(b, "Произошла ошибка при создании записи о контенте.", nil); replyErr != nil {
			log.Printf("Failed to send error message: %v", replyErr)
		}
		return handlers.EndConversation()
	}

	if _, err := msg.Reply(b, fmt.Sprintf("Запись о контенте '%s' с типом '%s' успешно создана с ID: %d", contentName, contentType, id), nil); err != nil {
		log.Printf("Error sending success message: %v", err)
	}

	// Clean up user data
	h.userStore.clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *contentSetupHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if _, err := msg.Reply(b, "Операция создания контента отменена.", nil); err != nil {
		log.Printf("Failed to send cancel message: %v", err)
	}

	// Clean up user data
	h.userStore.clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (s *setupUserDataStore) get(userID int64, key string) (any, bool) {
	s.rwMux.RLock()
	defer s.rwMux.RUnlock()

	userData, ok := s.userData[userID]
	if !ok {
		return nil, false
	}

	v, ok := userData[key]
	return v, ok
}

func (s *setupUserDataStore) set(userID int64, key string, val any) {
	s.rwMux.Lock()
	defer s.rwMux.Unlock()

	userData, ok := s.userData[userID]
	if !ok {
		userData = make(map[string]any)
		s.userData[userID] = userData
	}

	userData[key] = val
}

func (s *setupUserDataStore) clear(userID int64) {
	s.rwMux.Lock()
	defer s.rwMux.Unlock()

	delete(s.userData, userID)
}
