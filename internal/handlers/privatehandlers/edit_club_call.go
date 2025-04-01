package privatehandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/handlers" // Keep if Handler interface is used for registration
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const (
	EditClubCallHandlerName = "edit_club_call_handler"
	lastClubCallsLimitEdit  = 5

	// Manual states
	stateIdle           = ""
	stateWaitingCallID  = "wait_call_id"
	stateWaitingNewName = "wait_new_name"

	// Data keys
	dataKeyClubCallID = "club_call_id"
)

// EditClubCallHandler manages the conversation for editing club calls manually.
type EditClubCallHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config

	// In-memory state management
	userStates map[int64]string
	userData   map[int64]map[string]interface{}
	mu         sync.RWMutex
}

// NewEditClubCallHandler creates a new handler for editing club calls.
func NewEditClubCallHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) handlers.Handler { // Return the custom handlers.Handler interface
	return &EditClubCallHandler{
		contentRepository: contentRepository,
		config:            config,
		userStates:        make(map[int64]string),
		userData:          make(map[int64]map[string]interface{}),
	}
}

// getUserState safely retrieves the current state for a user.
func (h *EditClubCallHandler) getUserState(userID int64) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	state, ok := h.userStates[userID]
	if !ok {
		return stateIdle
	}
	return state
}

// setUserState safely sets the state for a user.
func (h *EditClubCallHandler) setUserState(userID int64, state string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if state == stateIdle {
		delete(h.userStates, userID)
		delete(h.userData, userID) // Clear data when returning to idle
	} else {
		h.userStates[userID] = state
	}
}

// getUserData safely retrieves data for a user.
func (h *EditClubCallHandler) getUserData(userID int64, key string) (interface{}, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data, userOk := h.userData[userID]
	if !userOk {
		return nil, false
	}
	val, keyOk := data[key]
	return val, keyOk
}

// setUserData safely sets data for a user.
func (h *EditClubCallHandler) setUserData(userID int64, key string, value interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, userOk := h.userData[userID]; !userOk {
		h.userData[userID] = make(map[string]interface{})
	}
	h.userData[userID][key] = value
}

// clearUserData safely removes all data for a user.
func (h *EditClubCallHandler) clearUserData(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.userData, userID)
}

// Name returns the handler name
func (h *EditClubCallHandler) Name() string {
	return EditClubCallHandlerName
}

// CheckUpdate determines if this handler should process the update.
func (h *EditClubCallHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.From == nil || msg.Chat.Type != constants.PrivateChat {
		return false
	}

	// Check if it's the entry command
	if strings.HasPrefix(msg.Text, constants.EditClubCallCommand) {
		return utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) // Check admin only for entry
	}

	// Check if the user is already in a conversation with this handler
	currentState := h.getUserState(msg.From.Id)
	return currentState != stateIdle
}

// HandleUpdate processes the update based on the user's current state.
func (h *EditClubCallHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	currentState := h.getUserState(userID)

	// Handle /cancel command anytime during the conversation
	if strings.ToLower(msg.Text) == "/cancel" {
		return h.cancel(b, ctx)
	}

	switch currentState {
	case stateIdle:
		// Should only be reached via the entry command (CheckUpdate handles this)
		if strings.HasPrefix(msg.Text, constants.EditClubCallCommand) {
			return h.startEdit(b, ctx)
		}
		// Ignore other messages if idle (shouldn't happen if CheckUpdate is correct)
		return nil
	case stateWaitingCallID:
		return h.handleWaitingCallID(b, ctx)
	case stateWaitingNewName:
		return h.handleWaitingNewName(b, ctx)
	default:
		// Should not happen, but reset state just in case
		log.Printf("User %d in unknown state: %s", userID, currentState)
		h.setUserState(userID, stateIdle)
		_, _ = msg.Reply(b, "Что-то пошло не так. Пожалуйста, начни заново с /editClubCall.", nil)
		return nil
	}
}

// --- State Handlers ---

// startEdit: Starts the conversation (Step 1)
func (h *EditClubCallHandler) startEdit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id

	// Admin check is already done in CheckUpdate for the entry command

	clubCalls, err := h.contentRepository.GetLastClubCalls(lastClubCallsLimitEdit)
	if err != nil {
		log.Printf("Failed to get last club calls for editing: %v", err)
		_, _ = msg.Reply(b, "Произошла ошибка при получении списка клубных звонков.", nil)
		h.setUserState(userID, stateIdle) // Reset state on error
		return nil
	}

	if len(clubCalls) == 0 {
		_, _ = msg.Reply(b, "Нет доступных клубных звонков для редактирования.", nil)
		h.setUserState(userID, stateIdle)
		return nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d клубных звонков:\n", len(clubCalls)))
	for _, call := range clubCalls {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s\n", call.ID, call.Name))
	}
	response.WriteString("\nПожалуйста, отправь ID клубного звонка, который ты хочешь отредактировать, или /cancel для отмены.")

	_, err = msg.Reply(b, response.String(), nil)
	if err != nil {
		log.Printf("Error sending club call list: %v", err)
		// Don't reset state yet, user might still reply
		return nil // Logged error, but continue
	}

	// Transition to the next state
	h.setUserState(userID, stateWaitingCallID)
	return nil
}

// handleWaitingCallID: Handles receiving the Club Call ID (Step 2)
func (h *EditClubCallHandler) handleWaitingCallID(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	callIDStr := msg.Text

	callID, err := strconv.Atoi(strings.TrimSpace(callIDStr))
	if err != nil {
		_, _ = msg.Reply(b, "Неверный ID. Пожалуйста, введи числовой ID или /cancel.", nil)
		return nil // Stay in the same state
	}

	// TODO: Validate if this ID exists in the database?

	// Store the selected ID
	h.setUserData(userID, dataKeyClubCallID, callID)

	_, err = msg.Reply(b, "Хорошо. Теперь введи новое название для этого клубного звонка, или /cancel для отмены.", nil)
	if err != nil {
		log.Printf("Error asking for new name: %v", err)
		// Don't reset state yet
		return nil
	}

	// Transition to the next state
	h.setUserState(userID, stateWaitingNewName)
	return nil
}

// handleWaitingNewName: Handles receiving the new name and updating (Steps 4 & 5)
func (h *EditClubCallHandler) handleWaitingNewName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		_, _ = msg.Reply(b, "Название не может быть пустым. Попробуй еще раз или /cancel.", nil)
		return nil // Stay in the same state
	}

	// Retrieve the call ID
	callIDVal, ok := h.getUserData(userID, dataKeyClubCallID)
	if !ok {
		log.Println("Error: club call ID not found in user data for user", userID)
		_, _ = msg.Reply(b, "Произошла внутренняя ошибка. Не удалось найти ID звонка. Попробуй начать заново с /editClubCall.", nil)
		h.setUserState(userID, stateIdle) // Reset state
		return nil
	}
	callID, ok := callIDVal.(int)
	if !ok {
		log.Printf("Error: club call ID in user data is not an int: %T for user %d", callIDVal, userID)
		_, _ = msg.Reply(b, "Произошла внутренняя ошибка (неверный тип ID). Попробуй начать заново с /editClubCall.", nil)
		h.setUserState(userID, stateIdle)
		return nil
	}

	// Update the name in the database
	err := h.contentRepository.UpdateContentName(callID, newName)
	if err != nil {
		log.Printf("Failed to update club call name for ID %d: %v", callID, err)
		if strings.Contains(err.Error(), "no content found") {
			_, _ = msg.Reply(b, fmt.Sprintf("Не удалось найти клубный звонок с ID %d для обновления.", callID), nil)
		} else {
			_, _ = msg.Reply(b, "Произошла ошибка при обновлении названия звонка в базе данных.", nil)
		}
		h.setUserState(userID, stateIdle) // Reset state on DB error
		return nil
	}

	_, err = msg.Reply(b, fmt.Sprintf("Название клубного звонка с ID %d успешно обновлено на '%s'.", callID, newName), nil)
	if err != nil {
		log.Printf("Error sending success message: %v", err)
	}

	// End the conversation successfully
	h.setUserState(userID, stateIdle)
	return nil
}

// cancel: Handler for cancellation
func (h *EditClubCallHandler) cancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id

	if h.getUserState(userID) != stateIdle {
		_, _ = msg.Reply(b, "Операция редактирования отменена.", nil)
		// Reset state and clear data
		h.setUserState(userID, stateIdle)
	} else {
		// If user wasn't in a conversation, just ignore /cancel
	}
	return nil
}
