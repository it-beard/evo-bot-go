package adminhandlers

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
	stateAskClubCallID = "ask_club_call_id"
	stateAskNewName    = "ask_new_name"

	// Context data keys
	ctxDataKeyClubCallID = "club_call_id"
)

// editClubCallHandler manages the conversation for editing club call names
type editClubCallHandler struct {
	contentRepo *repositories.ContentRepository
	config      *config.Config
	userStore   *userDataStore
}

// userDataStore provides thread-safe storage for user data during conversations
type userDataStore struct {
	rwMux    sync.RWMutex
	userData map[int64]map[string]any
}

// NewClubCallEditHandler creates a conversation handler for editing club calls
func NewClubCallEditHandler(
	contentRepo *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	store := &userDataStore{
		userData: make(map[int64]map[string]any),
	}

	h := &editClubCallHandler{
		contentRepo: contentRepo,
		config:      config,
		userStore:   store,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ClubCallEditCommand, h.startEdit),
		},
		map[string][]ext.Handler{
			stateAskClubCallID: {
				handlers.NewMessage(message.Text, h.handleClubCallID),
			},
			stateAskNewName: {
				handlers.NewMessage(message.Text, h.handleNewName),
			},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", h.handleCancel)},
			StateStorage: conversation.NewInMemoryStorage(conversation.KeyStrategySenderAndChat),
			AllowReEntry: true,
		},
	)
}

// startEdit is the entry point handler for the edit conversation
func (h *editClubCallHandler) startEdit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		if _, err := msg.Reply(b, "Эта команда доступна только администраторам.", nil); err != nil {
			log.Printf("Failed to send admin-only message: %v", err)
		}
		log.Printf("User %d tried to use %s without admin rights", msg.From.Id, constants.ClubCallEditCommand)
		return handlers.EndConversation()
	}

	if msg.Chat.Type != constants.PrivateChatType {
		if _, err := msg.Reply(b, "Эта команда доступна только в личном чате.", nil); err != nil {
			log.Printf("Failed to send private-only message: %v", err)
		}
		return handlers.EndConversation()
	}

	// Get club calls for editing
	clubCalls, err := h.contentRepo.GetLastClubCalls(constants.ClubCallEditGetLastLimit)
	if err != nil {
		log.Printf("Failed to get last club calls for editing: %v", err)
		if _, err := msg.Reply(b, "Произошла ошибка при получении списка клубных звонков.", nil); err != nil {
			log.Printf("Failed to send error message: %v", err)
		}
		return handlers.EndConversation()
	}

	if len(clubCalls) == 0 {
		if _, err := msg.Reply(b, "Нет доступных клубных звонков для редактирования.", nil); err != nil {
			log.Printf("Failed to send no available calls message: %v", err)
		}
		return handlers.EndConversation()
	}

	// Build response with available club calls
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d клубных звонков:\n", len(clubCalls)))
	for _, call := range clubCalls {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s\n", call.ID, call.Name))
	}
	response.WriteString("\nПожалуйста, отправь ID клубного звонка, который ты хочешь отредактировать, или /cancel для отмены.")

	if _, err := msg.Reply(b, response.String(), nil); err != nil {
		log.Printf("Error sending club call list: %v", err)
	}

	return handlers.NextConversationState(stateAskClubCallID)
}

// handleClubCallID processes the user's selected club call ID
func (h *editClubCallHandler) handleClubCallID(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	callIDStr := strings.TrimSpace(msg.Text)

	callID, err := strconv.Atoi(callIDStr)
	if err != nil {
		if _, err := msg.Reply(b, "Неверный ID. Пожалуйста, введи числовой ID или /cancel.", nil); err != nil {
			log.Printf("Failed to send invalid ID message: %v", err)
		}
		return nil // Stay in the same state
	}

	h.userStore.set(ctx.EffectiveUser.Id, ctxDataKeyClubCallID, callID)

	if _, err := msg.Reply(b, "Хорошо. Теперь введи новое название для этого клубного звонка, или /cancel для отмены.", nil); err != nil {
		log.Printf("Error asking for new name: %v", err)
	}

	return handlers.NextConversationState(stateAskNewName)
}

// handleNewName processes the new name and updates the club call
func (h *editClubCallHandler) handleNewName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		if _, err := msg.Reply(b, "Название не может быть пустым. Попробуй еще раз или /cancel.", nil); err != nil {
			log.Printf("Failed to send empty name error: %v", err)
		}
		return nil // Stay in the same state
	}

	// Get the call ID from user data store
	callIDVal, ok := h.userStore.get(ctx.EffectiveUser.Id, ctxDataKeyClubCallID)
	if !ok {
		log.Printf("Error: club call ID not found in user data for user %d", ctx.EffectiveUser.Id)
		if _, err := msg.Reply(b, "Произошла внутренняя ошибка. Не удалось найти ID звонка. Попробуй начать заново с /editClubCall.", nil); err != nil {
			log.Printf("Failed to send error message: %v", err)
		}
		return handlers.EndConversation()
	}

	callID, ok := callIDVal.(int)
	if !ok {
		log.Printf("Error: club call ID in user data is not an int: %T. Value: %v", callIDVal, callIDVal)
		if _, err := msg.Reply(b, "Произошла внутренняя ошибка (неверный тип ID). Попробуй начать заново с /editClubCall.", nil); err != nil {
			log.Printf("Failed to send type error message: %v", err)
		}
		return handlers.EndConversation()
	}

	// Update the name in the database
	err := h.contentRepo.UpdateContentName(callID, newName)
	if err != nil {
		log.Printf("Failed to update club call name for ID %d: %v", callID, err)
		errorMsg := "Произошла ошибка при обновлении названия звонка в базе данных."
		if strings.Contains(err.Error(), "no content found") {
			errorMsg = fmt.Sprintf("Не удалось найти клубный звонок с ID %d для обновления.", callID)
		}
		if _, err := msg.Reply(b, errorMsg, nil); err != nil {
			log.Printf("Failed to send update error message: %v", err)
		}
		return handlers.EndConversation()
	}

	if _, err := msg.Reply(b, fmt.Sprintf("Название клубного звонка с ID %d успешно обновлено на '%s'.", callID, newName), nil); err != nil {
		log.Printf("Error sending success message: %v", err)
	}

	// Clean up user data
	h.userStore.clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCancel handles the /cancel command
func (h *editClubCallHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if _, err := msg.Reply(b, "Операция редактирования отменена.", nil); err != nil {
		log.Printf("Failed to send cancel message: %v", err)
	}

	// Clean up user data
	h.userStore.clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// User data store methods
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
