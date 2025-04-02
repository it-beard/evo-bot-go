package contenthandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/conversation"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	stateAskContentID = "ask_content_id"
	stateAskNewName   = "ask_new_name"

	// Context data keys
	ctxDataKeyContentID = "content_id"
	cancelCommand       = "cancel"
)

type contentEditHandler struct {
	contentRepo *repositories.ContentRepository
	config      *config.Config
	userStore   *userDataStore
}

type userDataStore struct {
	rwMux    sync.RWMutex
	userData map[int64]map[string]any
}

func NewContentEditHandler(
	contentRepo *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	store := &userDataStore{
		userData: make(map[int64]map[string]any),
	}

	h := &contentEditHandler{
		contentRepo: contentRepo,
		config:      config,
		userStore:   store,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentEditCommand, h.startEdit),
		},
		map[string][]ext.Handler{
			stateAskContentID: {
				handlers.NewMessage(message.Text, h.handleContentID),
			},
			stateAskNewName: {
				handlers.NewMessage(message.Text, h.handleNewName),
			},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand(cancelCommand, h.handleCancel)},
			StateStorage: conversation.NewInMemoryStorage(conversation.KeyStrategySenderAndChat),
			AllowReEntry: true,
		},
	)
}

// 1. startEdit is the entry point handler for the edit conversation
func (h *contentEditHandler) startEdit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		if _, err := msg.Reply(b, "Эта команда доступна только администраторам.", nil); err != nil {
			log.Printf("Failed to send admin-only message: %v", err)
		}
		log.Printf("User %d tried to use %s without admin rights", msg.From.Id, constants.ContentEditCommand)
		return handlers.EndConversation()
	}

	// Check if the command is used in a private chat
	if msg.Chat.Type != constants.PrivateChatType {
		if _, err := msg.Reply(b, "Эта команда доступна только в личном чате.", nil); err != nil {
			log.Printf("Failed to send private-only message: %v", err)
		}
		return handlers.EndConversation()
	}

	// Get contents for editing
	contents, err := h.contentRepo.GetLastContents(constants.ContentEditGetLastLimit)
	if err != nil {
		log.Printf("Failed to get last contents for editing: %v", err)
		if _, err := msg.Reply(b, "Произошла ошибка при получении списка контента.", nil); err != nil {
			log.Printf("Failed to send error message: %v", err)
		}
		return handlers.EndConversation()
	}

	if len(contents) == 0 {
		if _, err := msg.Reply(b, "Нет доступных контента для редактирования.", nil); err != nil {
			log.Printf("Failed to send no available contents message: %v", err)
		}
		return handlers.EndConversation()
	}

	// Build response with available contents
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d контента:\n", len(contents)))
	for _, content := range contents {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s\n", content.ID, content.Name))
	}
	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID контента, который ты хочешь отредактировать, или /%s для отмены.", cancelCommand))

	if _, err := msg.Reply(b, response.String(), nil); err != nil {
		log.Printf("Error sending content list: %v", err)
	}

	return handlers.NextConversationState(stateAskContentID)
}

// 2. handleContentID processes the user's selected content ID
func (h *contentEditHandler) handleContentID(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	contentIDStr := strings.TrimSpace(msg.Text)

	contentID, err := strconv.Atoi(contentIDStr)
	if err != nil {
		if _, err := msg.Reply(b, "Неверный ID. Пожалуйста, введи числовой ID или /cancel.", nil); err != nil {
			log.Printf("Failed to send invalid ID message: %v", err)
		}
		return nil // Stay in the same state
	}

	h.userStore.set(ctx.EffectiveUser.Id, ctxDataKeyContentID, contentID)

	if _, err := msg.Reply(b, fmt.Sprintf("Хорошо. Теперь введи новое название для этого контента, или /%s для отмены.", cancelCommand), nil); err != nil {
		log.Printf("Error asking for new name: %v", err)
	}

	return handlers.NextConversationState(stateAskNewName)
}

// 3. handleNewName processes the new name and updates the content
func (h *contentEditHandler) handleNewName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		if _, err := msg.Reply(b, fmt.Sprintf("Название не может быть пустым. Попробуй еще раз или /%s для отмены.", cancelCommand), nil); err != nil {
			log.Printf("Failed to send empty name error: %v", err)
		}
		return nil // Stay in the same state
	}

	// Get the content ID from user data store
	contentIDVal, ok := h.userStore.get(ctx.EffectiveUser.Id, ctxDataKeyContentID)
	if !ok {
		log.Printf("Error: content ID not found in user data for user %d", ctx.EffectiveUser.Id)
		if _, err := msg.Reply(b, fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти ID контента. Попробуй начать заново с /%s.", constants.ContentEditCommand), nil); err != nil {
			log.Printf("Failed to send error message: %v", err)
		}
		return handlers.EndConversation()
	}

	contentID, ok := contentIDVal.(int)
	if !ok {
		log.Printf("Error: content ID in user data is not an int: %T. Value: %v", contentIDVal, contentIDVal)
		if _, err := msg.Reply(b, fmt.Sprintf("Произошла внутренняя ошибка (неверный тип ID). Попробуй начать заново с /%s.", constants.ContentEditCommand), nil); err != nil {
			log.Printf("Failed to send type error message: %v", err)
		}
		return handlers.EndConversation()
	}

	// Update the name in the database
	err := h.contentRepo.UpdateContentName(contentID, newName)
	if err != nil {
		log.Printf("Failed to update content name for ID %d: %v", contentID, err)
		errorMsg := "Произошла ошибка при обновлении названия контента в базе данных."
		if strings.Contains(err.Error(), "no content found") {
			errorMsg = fmt.Sprintf("Не удалось найти контент с ID %d для обновления.", contentID)
		}
		if _, err := msg.Reply(b, errorMsg, nil); err != nil {
			log.Printf("Failed to send update error message: %v", err)
		}
		return handlers.EndConversation()
	}

	if _, err := msg.Reply(b, fmt.Sprintf("Название контента с ID %d успешно обновлено на '%s'.", contentID, newName), nil); err != nil {
		log.Printf("Error sending success message: %v", err)
	}

	// Clean up user data
	h.userStore.clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *contentEditHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if _, err := msg.Reply(b, "Операция редактирования отменена.", nil); err != nil {
		log.Printf("Failed to send cancel message: %v", err)
	}

	// Clean up user data
	h.userStore.clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (s *userDataStore) get(userID int64, key string) (any, bool) {
	s.rwMux.RLock()
	defer s.rwMux.RUnlock()

	userData, ok := s.userData[userID]
	if !ok {
		return nil, false
	}

	v, ok := userData[key]
	return v, ok
}

func (s *userDataStore) set(userID int64, key string, val any) {
	s.rwMux.Lock()
	defer s.rwMux.Unlock()

	userData, ok := s.userData[userID]
	if !ok {
		userData = make(map[string]any)
		s.userData[userID] = userData
	}

	userData[key] = val
}

func (s *userDataStore) clear(userID int64) {
	s.rwMux.Lock()
	defer s.rwMux.Unlock()

	delete(s.userData, userID)
}
