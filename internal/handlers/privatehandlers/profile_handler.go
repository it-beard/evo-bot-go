package privatehandlers

import (
	"database/sql"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	profileStateViewOptions   = "profile_state_view_options"
	profileStateEditMyProfile = "profile_state_edit_my_profile"
	profileStateAwaitUsername = "profile_state_await_username"
	profileStateAwaitBio      = "profile_state_await_bio"
	profileStateAwaitLinkedin = "profile_state_await_linkedin"
	profileStateAwaitGithub   = "profile_state_await_github"
	profileStateAwaitWebsite  = "profile_state_await_website"

	// UserStore keys
	profileCtxDataKeyField             = "profile_ctx_data_field"
	profileCtxDataKeyPreviousMessageID = "profile_ctx_data_previous_message_id"
	profileCtxDataKeyPreviousChatID    = "profile_ctx_data_previous_chat_id"
)

type profileHandler struct {
	config               *config.Config
	messageSenderService *services.MessageSenderService
	permissionsService   *services.PermissionsService
	userRepository       *repositories.UserRepository
	profileRepository    *repositories.ProfileRepository
	userStore            *utils.UserDataStore
}

func NewProfileHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
	userRepository *repositories.UserRepository,
	profileRepository *repositories.ProfileRepository,
) ext.Handler {
	h := &profileHandler{
		config:               config,
		messageSenderService: messageSenderService,
		permissionsService:   permissionsService,
		userRepository:       userRepository,
		profileRepository:    profileRepository,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ProfileCommand, h.handleCommand),
		},
		map[string][]ext.Handler{
			profileStateViewOptions: {
				handlers.NewCallback(callbackquery.Prefix(constants.ProfilePrefix), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateEditMyProfile: {
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
			},
			profileStateAwaitUsername: {
				handlers.NewMessage(message.Text, h.handleUsernameInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitBio: {
				handlers.NewMessage(message.Text, h.handleBioInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitLinkedin: {
				handlers.NewMessage(message.Text, h.handleLinkedinInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitGithub: {
				handlers.NewMessage(message.Text, h.handleGithubInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitWebsite: {
				handlers.NewMessage(message.Text, h.handleWebsiteInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

func (h *profileHandler) showProfileMenu(b *gotgbot.Bot, msg *gotgbot.Message, userId int64) error {
	h.RemovePreviouseMessage(b, &userId)

	editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(
		msg.Chat.Id,
		"*Профиль*\n\nВыберите действие:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileMainButtons(),
		})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

// Entry point for the /profile command
func (h *profileHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.ProfileCommand) {
		return handlers.EndConversation()
	}

	return h.showProfileMenu(b, msg, ctx.EffectiveUser.Id)
}

// Handles button clicks
func (h *profileHandler) handleCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	callback := ctx.Update.CallbackQuery
	data := callback.Data

	effectiveMsg := ctx.EffectiveMessage
	userId := callback.From.Id

	// Acknowledge the callback to stop loading animation
	_, err := callback.Answer(b, nil)
	if err != nil {
		return fmt.Errorf("failed to answer callback: %w", err)
	}

	switch data {
	case constants.ProfileViewMyProfileCallback:
		return h.handleViewMyProfile(b, ctx, effectiveMsg)
	case constants.ProfileEditMyProfileCallback:
		return h.handleEditMyProfile(b, ctx, effectiveMsg)
	case constants.ProfileViewOtherProfileCallback:
		return h.handleViewOtherProfile(b, ctx, effectiveMsg)
	case constants.ProfileEditBioCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "биографию (до 2000 символов)", profileStateAwaitBio)
	case constants.ProfileEditLinkedinCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "ссылку на LinkedIn", profileStateAwaitLinkedin)
	case constants.ProfileEditGithubCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "ссылку на GitHub", profileStateAwaitGithub)
	case constants.ProfileEditWebsiteCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "ссылку на ваш ресурс", profileStateAwaitWebsite)
	case constants.ProfileStartCallback:
		return h.showProfileMenu(b, effectiveMsg, userId)
	}

	return nil
}

func (h *profileHandler) handleViewMyProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	// Get or create user in our DB
	user := ctx.Update.CallbackQuery.From
	dbUser, err := h.getOrCreateUser(&user)
	if err != nil {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении информации о пользователе.", nil)
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Try to get profile
	profile, err := h.profileRepository.GetByUserID(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении профиля.", nil)
		return fmt.Errorf("failed to get profile: %w", err)
	}

	profileText := formatters.FormatProfileView(dbUser, profile, true)
	editedMsg, err := h.messageSenderService.SendWithReturnMessage(msg.Chat.Id, profileText,
		&gotgbot.SendMessageOpts{
			ParseMode:   "html",
			ReplyMarkup: formatters.ProfileEditBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.RemovePreviouseMessage(b, &user.Id)
	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

func (h *profileHandler) handleEditMyProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	currentUser := ctx.Update.CallbackQuery.From
	_, err := h.getOrCreateUser(&currentUser)
	if err != nil {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении информации о пользователе.", nil)
		return fmt.Errorf("failed to get user: %w", err)
	}

	h.RemovePreviouseMessage(b, &currentUser.Id)
	editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(
		msg.Chat.Id,
		"*Редактирование профиля*"+
			"\n\nВыберите, что хотите изменить:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileEditButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(currentUser.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

func (h *profileHandler) handleViewOtherProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	user := ctx.Update.CallbackQuery.From

	h.RemovePreviouseMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(
		msg.Chat.Id,
		"*Поиск профиля*\n\nВведите имя пользователя (с @ или без):",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileSearchBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateAwaitUsername)
}

func (h *profileHandler) handleUsernameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	username := msg.Text
	userId := ctx.EffectiveMessage.From.Id

	// Remove @ if present
	username = strings.TrimPrefix(username, "@")

	if username == "" {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			"Пожалуйста, введите корректное имя пользователя.", nil)
		return nil // Stay in the same state
	}

	// Try to get user
	dbUser, err := h.userRepository.GetByTelegramUsername(username)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении информации о пользователе.", nil)
		return fmt.Errorf("failed to get user: %w", err)
	}

	// If user not found, show search again
	if err == sql.ErrNoRows {
		h.RemovePreviouseMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
			fmt.Sprintf("*Поиск профиля*\n\nПользователь *%s* не найден.", username)+
				"\n\nНажмите на кнопку \"🔎 Ещё раз\" для повторного поиска.",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileSearchBackCancelButtons(constants.ProfileStartCallback),
			})
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		h.SavePreviousMessageInfo(userId, editedMsg)
		return handlers.NextConversationState(profileStateViewOptions)
	}

	// Try to get profile
	profile, err := h.profileRepository.GetByUserID(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении профиля.", nil)
		return fmt.Errorf("failed to get profile: %w", err)
	}

	// Format the profile view
	profileText := formatters.FormatProfileView(dbUser, profile, false)
	editedMsg, err := h.messageSenderService.ReplyWithReturnMessage(msg, profileText,
		&gotgbot.SendMessageOpts{
			ParseMode:   "html",
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(ctx.EffectiveMessage.From.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

// Starts the process of editing a specific profile field
func (h *profileHandler) handleEditField(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message, fieldName string, nextState string) error {
	user := ctx.Update.CallbackQuery.From
	oldFieldValue := ""

	h.userStore.Set(user.Id, profileCtxDataKeyField, fieldName)

	dbUser, err := h.userRepository.GetByTelegramID(user.Id)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if dbUser != nil {
		dbProfile, err := h.profileRepository.GetByUserID(dbUser.ID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get profile: %w", err)
		}

		if dbProfile != nil {
			switch nextState {
			case profileStateAwaitBio:
				oldFieldValue = dbProfile.Bio
			case profileStateAwaitLinkedin:
				oldFieldValue = dbProfile.LinkedIn
			case profileStateAwaitGithub:
				oldFieldValue = dbProfile.GitHub
			case profileStateAwaitWebsite:
				oldFieldValue = dbProfile.Website
			}
		}
		if oldFieldValue == "" || oldFieldValue == " " {
			oldFieldValue = "отсутствует"
		}
	}

	h.RemovePreviouseMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(
		msg.Chat.Id,
		"*Редактирование профиля*"+
			fmt.Sprintf("\n\nТекущее значение: `%s`", oldFieldValue)+
			fmt.Sprintf("\n\nВведите новую %s:", fieldName),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(nextState)
}

// Bio handler
func (h *profileHandler) handleBioInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	bio := msg.Text

	if len(bio) > 2000 {
		h.RemovePreviouseMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			"Биография слишком длинная. Пожалуйста, сократите до 2000 символов и пришлите снова:", nil)

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "bio", bio)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			"Произошла ошибка при сохранении биографии.", nil)
		return fmt.Errorf("failed to save bio: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		"✅ Биография сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// LinkedIn handler
func (h *profileHandler) handleLinkedinInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	linkedin := msg.Text

	if !strings.Contains(linkedin, "linkedin") {
		h.RemovePreviouseMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			"Пожалуйста, введите корректную ссылку на LinkedIn:", nil)

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "linkedin", linkedin)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			"Произошла ошибка при сохранении ссылки на LinkedIn.", nil)
		return fmt.Errorf("failed to save LinkedIn: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(
		msg.Chat.Id,
		"✅ Ссылка на LinkedIn сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// GitHub handler
func (h *profileHandler) handleGithubInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	github := msg.Text

	if !strings.Contains(github, "github") {
		h.RemovePreviouseMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			"Пожалуйста, введите корректную ссылку на GitHub:", nil)

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "github", github)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			"Произошла ошибка при сохранении ссылки на GitHub.", nil)
		return fmt.Errorf("failed to save GitHub: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		"✅ Ссылка на GitHub сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// Website handler
func (h *profileHandler) handleWebsiteInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	website := msg.Text

	if !strings.Contains(website, "http") {
		h.RemovePreviouseMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			"Пожалуйста, введите корректную ссылку:", nil)

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "website", website)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			"Произошла ошибка при сохранении ссылки на вебсайт.", nil)
		return fmt.Errorf("failed to save website: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		"✅ Ссылка сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

func (h *profileHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

func (h *profileHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	_ = h.messageSenderService.Reply(msg, "Диалог о профилях завершен.", nil)
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *profileHandler) getOrCreateUser(tgUser *gotgbot.User) (*repositories.User, error) {
	// Try to get user by Telegram ID
	dbUser, err := h.userRepository.GetByTelegramID(int64(tgUser.Id))
	if err == nil {
		// User exists, return it
		return dbUser, nil
	}

	// If error is not "no rows", it's a real error
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	// User doesn't exist, create new user
	userID, err := h.userRepository.Create(
		int64(tgUser.Id),
		tgUser.FirstName,
		tgUser.LastName,
		tgUser.Username,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	// Get the newly created user
	dbUser, err = h.userRepository.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("error getting created user: %w", err)
	}

	return dbUser, nil
}

func (h *profileHandler) saveProfileField(tgUser *gotgbot.User, fieldName string, value string) error {
	// Get or create user
	dbUser, err := h.getOrCreateUser(tgUser)
	if err != nil {
		return fmt.Errorf("error getting/creating user: %w", err)
	}

	// Try to get profile
	profile, err := h.profileRepository.GetByUserID(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting profile: %w", err)
	}

	// If profile doesn't exist, create it
	if err == sql.ErrNoRows {
		// Initialize defaults for all fields
		bio := ""
		linkedin := ""
		github := ""
		website := ""

		// Set the value for the current field
		switch fieldName {
		case "bio":
			bio = value
		case "linkedin":
			linkedin = value
		case "github":
			github = value
		case "website":
			website = value
		}

		// Create new profile
		_, err = h.profileRepository.Create(dbUser.ID, bio, linkedin, github, website)
		if err != nil {
			return fmt.Errorf("error creating profile: %w", err)
		}

		return nil
	}

	// Profile exists, update the specific field
	fields := map[string]interface{}{
		fieldName: value,
	}

	// Update profile
	err = h.profileRepository.Update(profile.ID, fields)
	if err != nil {
		return fmt.Errorf("error updating profile: %w", err)
	}

	return nil
}

func (h *profileHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			profileCtxDataKeyPreviousMessageID,
			profileCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *profileHandler) RemovePreviouseMessage(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			profileCtxDataKeyPreviousMessageID,
			profileCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	b.DeleteMessage(chatID, messageID, nil)
}

func (h *profileHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		profileCtxDataKeyPreviousMessageID, profileCtxDataKeyPreviousChatID)
}
