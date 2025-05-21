package adminhandlers

import (
	"database/sql"
	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
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
	adminProfilesStateStart               = "admin_profiles_state_start"
	adminProfilesStateEdit                = "admin_profiles_state_edit"
	adminProfilesStateAwaitUsername       = "admin_profiles_state_await_username"
	adminProfilesStateAwaitForwardMessage = "admin_profiles_state_await_forward_message"
	adminProfilesStateEditProfile         = "admin_profiles_state_edit_profile"
	adminProfilesStateAwaitBio            = "admin_profiles_state_await_bio"
	adminProfilesStateAwaitFirstname      = "admin_profiles_state_await_firstname"
	adminProfilesStateAwaitLastname       = "admin_profiles_state_await_lastname"
	adminProfilesStateAwaitCoffeeBan      = "admin_profiles_state_await_coffee_ban"

	// UserStore keys
	adminProfilesCtxDataKeyField             = "admin_profiles_ctx_data_field"
	adminProfilesCtxDataKeyUserID            = "admin_profiles_ctx_data_user_id"
	adminProfilesCtxDataKeyPreviousMessageID = "admin_profiles_ctx_data_previous_message_id"
	adminProfilesCtxDataKeyPreviousChatID    = "admin_profiles_ctx_data_previous_chat_id"
	adminProfilesCtxDataKeyProfileID         = "admin_profiles_ctx_data_profile_id"
	adminProfilesCtxDataKeyTelegramID        = "admin_profiles_ctx_data_telegram_id"
	adminProfilesCtxDataKeyTelegramUsername  = "admin_profiles_ctx_data_telegram_username"

	// Menu headers
	adminProfilesMenuHeader              = "Админ-меню \"Менеджер профилей\""
	adminProfilesMenuEditHeader          = "Менеджер профилей → Редактирование"
	adminProfilesMenuEditFirstnameHeader = "Менеджер профилей → Редактирование → Имя"
	adminProfilesMenuEditLastnameHeader  = "Менеджер профилей → Редактирование → Фамилия"
	adminProfilesMenuEditBioHeader       = "Менеджер профилей → Редактирование → О себе"
	adminProfilesMenuPublishHeader       = "Менеджер профилей → Публикация"
	adminProfilesMenuCoffeeBanHeader     = "Менеджер профилей → Бан на кофейные встречи"

	// Other
	adminProfilesBioLengthLimit = 4000
)

type adminProfilesHandler struct {
	config               *config.Config
	messageSenderService *services.MessageSenderService
	permissionsService   *services.PermissionsService
	userRepository       *repositories.UserRepository
	profileRepository    *repositories.ProfileRepository
	userStore            *utils.UserDataStore
}

func NewAdminProfilesHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
	userRepository *repositories.UserRepository,
	profileRepository *repositories.ProfileRepository,
) ext.Handler {
	h := &adminProfilesHandler{
		config:               config,
		messageSenderService: messageSenderService,
		permissionsService:   permissionsService,
		userRepository:       userRepository,
		profileRepository:    profileRepository,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.AdminProfilesCommand, h.handleCommand),
		},
		map[string][]ext.Handler{
			adminProfilesStateStart: {
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesEditCallback), h.handleEditCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCreateCallback), h.handleCreateCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
			},
			adminProfilesStateAwaitUsername: {
				handlers.NewMessage(message.Text, h.handleUsernameInput),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesStartCallback), h.handleStartCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
			},
			adminProfilesStateAwaitForwardMessage: {
				handlers.NewMessage(message.All, h.handleForwardedMessage),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesStartCallback), h.handleStartCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
			},
			adminProfilesStateEditProfile: {
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesEditBioCallback), h.handleEditFieldCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesEditFirstnameCallback), h.handleEditFieldCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesEditLastnameCallback), h.handleEditFieldCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesEditCoffeeBanCallback), h.handleEditFieldCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesPublishCallback), h.handlePublishCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesPublishNoPreviewCallback), h.handlePublishNoPreviewCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesStartCallback), h.handleStartCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
			},
			adminProfilesStateAwaitBio: {
				handlers.NewMessage(message.Text, h.handleBioInput),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesStartCallback), h.handleStartCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
			},
			adminProfilesStateAwaitFirstname: {
				handlers.NewMessage(message.Text, h.handleFirstnameInput),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesStartCallback), h.handleStartCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
			},
			adminProfilesStateAwaitLastname: {
				handlers.NewMessage(message.Text, h.handleLastnameInput),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesStartCallback), h.handleStartCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
			},
			adminProfilesStateAwaitCoffeeBan: {
				handlers.NewMessage(message.Text, h.handleCoffeeBanInput),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesStartCallback), h.handleStartCallback),
				handlers.NewCallback(callbackquery.Equal(constants.AdminProfilesCancelCallback), h.handleCancelCallback),
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

// Entry point for the /profiles command
func (h *adminProfilesHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.AdminProfilesCommand) {
		return handlers.EndConversation()
	}

	return h.showMainMenu(b, ctx.EffectiveMessage, ctx.EffectiveUser.Id)
}

// Shows the main profiles menu for admin
func (h *adminProfilesHandler) showMainMenu(b *gotgbot.Bot, msg *gotgbot.Message, userId int64) error {
	h.RemovePreviousMessage(b, &userId)

	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", adminProfilesMenuHeader)+
			"\n\nЗдесь ты можешь редактировать профили пользователей или создать новый профиль на основе пересланного сообщения.",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfilesMainMenuButtons(),
		})

	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to send message in showMainMenu: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(adminProfilesStateStart)
}

// Handle the "Edit profile" button click
func (h *adminProfilesHandler) handleEditCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	h.RemovePreviousMessage(b, &userId)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", adminProfilesMenuHeader)+
			"\n\nВведи имя пользователя (с @ или без) для поиска:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
		})

	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to send message in handleCallbackEdit: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(adminProfilesStateAwaitUsername)
}

// Handle the "Create profile" button click
func (h *adminProfilesHandler) handleCreateCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	h.RemovePreviousMessage(b, &userId)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", adminProfilesMenuHeader)+
			"\n\nПерешли мне сообщение от пользователя, для которого нужно создать профиль:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
		})

	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to send message in handleCallbackCreate: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(adminProfilesStateAwaitForwardMessage)
}

// Handle the "Start" button click - goes back to the main menu
func (h *adminProfilesHandler) handleStartCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return h.showMainMenu(b, ctx.EffectiveMessage, ctx.EffectiveUser.Id)
}

// Handle the username input for profile search
func (h *adminProfilesHandler) handleUsernameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	username := msg.Text
	userId := ctx.EffectiveUser.Id

	// Remove @ if present
	username = strings.TrimPrefix(username, "@")

	dbUser, err := h.userRepository.GetByTelegramUsername(username)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("AdminProfilesHandler: failed to get user in handleUsernameInput: %w", err)
	}

	// If user not found, show search again
	if err == sql.ErrNoRows {
		h.RemovePreviousMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", adminProfilesMenuHeader)+
				fmt.Sprintf("\n\nПользователь <b>%s</b> не найден.", username)+
				"\n\nПопробуй ещё раз, или вернись назад:",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
			})
		if err != nil {
			return fmt.Errorf("AdminProfilesHandler: failed to send message in handleUsernameInput: %w", err)
		}

		h.SavePreviousMessageInfo(userId, editedMsg)
		return nil
	}

	// Store the user ID for future use
	h.userStore.Set(userId, adminProfilesCtxDataKeyUserID, dbUser.ID)
	h.userStore.Set(userId, adminProfilesCtxDataKeyTelegramID, dbUser.TgID)
	h.userStore.Set(userId, adminProfilesCtxDataKeyTelegramUsername, dbUser.TgUsername)

	// Find or create the profile
	profile, err := h.profileRepository.GetOrCreateDefaultProfile(dbUser.ID)
	if err != nil {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении или создании профиля.", nil)
		return fmt.Errorf("AdminProfilesHandler: failed to get/create profile in handleUsernameInput: %w", err)
	}

	h.userStore.Set(userId, adminProfilesCtxDataKeyProfileID, profile.ID)

	// Delete the input message
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	h.RemovePreviousMessage(b, &userId)

	// Show the profile edit menu
	return h.showProfileEditMenu(b, msg, userId, dbUser, profile)
}

// Handle forwarded message for profile creation
func (h *adminProfilesHandler) handleForwardedMessage(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	// Check if this is a forwarded message with user origin
	if msg.ForwardOrigin == nil || msg.ForwardOrigin.GetType() != "user" {
		h.RemovePreviousMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", adminProfilesMenuHeader)+
				"\n\nЭто не пересланное сообщение от пользователя. Пожалуйста, перешли сообщение от пользователя, для которого нужно создать профиль:",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
			})

		if err != nil {
			return fmt.Errorf("AdminProfilesHandler: failed to send message in handleForwardedMessage: %w", err)
		}

		h.SavePreviousMessageInfo(userId, editedMsg)
		return nil // Stay in the current state
	}

	// Cast ForwardOrigin to MessageOriginUser to get user info
	forwardedUser := msg.ForwardOrigin.MergeMessageOrigin().SenderUser

	// Get the user from the database if exists, or create a new one
	dbUser, err := h.userRepository.GetOrCreateUser(forwardedUser)
	if err != nil {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при создании пользователя.", nil)
		return fmt.Errorf("AdminProfilesHandler: failed to create user in handleForwardedMessage: %w", err)
	}

	// Store the user ID for future use
	h.userStore.Set(userId, adminProfilesCtxDataKeyUserID, dbUser.ID)
	h.userStore.Set(userId, adminProfilesCtxDataKeyTelegramID, dbUser.TgID)
	h.userStore.Set(userId, adminProfilesCtxDataKeyTelegramUsername, dbUser.TgUsername)

	// Find or create the profile
	profile, err := h.profileRepository.GetOrCreateDefaultProfileWithBio(dbUser.ID, msg.Text)
	if err != nil {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при создании профиля.", nil)
		return fmt.Errorf("AdminProfilesHandler: failed to create profile in handleForwardedMessage: %w", err)
	}

	h.userStore.Set(userId, adminProfilesCtxDataKeyProfileID, profile.ID)

	// Delete the forwarded message
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	h.RemovePreviousMessage(b, &userId)

	// Show the profile edit menu
	return h.showProfileEditMenu(b, msg, userId, dbUser, profile)
}

// Shows the profile edit menu
func (h *adminProfilesHandler) showProfileEditMenu(b *gotgbot.Bot, msg *gotgbot.Message, userId int64, user *repositories.User, profile *repositories.Profile) error {
	profileText := fmt.Sprintf("<b>%s</b>\n\n%s", adminProfilesMenuEditHeader, formatters.FormatProfileManagerView(user, profile, user.HasCoffeeBan))

	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		profileText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfilesEditMenuButtons(constants.AdminProfilesStartCallback),
		})

	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to send message in showProfileEditMenu: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(adminProfilesStateEditProfile)
}

// Handle button clicks for editing different profile fields
func (h *adminProfilesHandler) handleEditFieldCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	callback := ctx.Update.CallbackQuery
	data := callback.Data
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	// Get stored user and profile IDs
	userIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyUserID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: user ID not found in user store")
	}
	dbUserID := userIDVal.(int)

	// Get the user from database
	dbUser, err := h.userRepository.GetByID(dbUserID)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to get user in handleCallbackEditField: %w", err)
	}

	// Get the profile from database
	profileIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyProfileID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: profile ID not found in user store")
	}
	profileID := profileIDVal.(int)

	profile, err := h.profileRepository.GetByID(profileID)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to get profile in handleCallbackEditField: %w", err)
	}

	var fieldName string
	var menuHeader string
	var nextState string
	var oldFieldValue string

	// Determine which field is being edited
	switch data {
	case constants.AdminProfilesEditFirstnameCallback:
		fieldName = "имя"
		menuHeader = adminProfilesMenuEditFirstnameHeader
		nextState = adminProfilesStateAwaitFirstname
		oldFieldValue = dbUser.Firstname
	case constants.AdminProfilesEditLastnameCallback:
		fieldName = "фамилию"
		menuHeader = adminProfilesMenuEditLastnameHeader
		nextState = adminProfilesStateAwaitLastname
		oldFieldValue = dbUser.Lastname
	case constants.AdminProfilesEditBioCallback:
		fieldName = fmt.Sprintf("биографию (до %d символов)", adminProfilesBioLengthLimit)
		menuHeader = adminProfilesMenuEditBioHeader
		nextState = adminProfilesStateAwaitBio
		oldFieldValue = profile.Bio
	case constants.AdminProfilesEditCoffeeBanCallback:
		fieldName = "статус кофейных встреч"
		menuHeader = adminProfilesMenuCoffeeBanHeader
		nextState = adminProfilesStateAwaitCoffeeBan
		if dbUser.HasCoffeeBan {
			oldFieldValue = "❌ Запрещено"
		} else {
			oldFieldValue = "✅ Разрешено"
		}
	default:
		return fmt.Errorf("AdminProfilesHandler: unknown callback data: %s", data)
	}

	// Store field being edited for use in input handlers
	h.userStore.Set(userId, adminProfilesCtxDataKeyField, data)

	if oldFieldValue == "" || oldFieldValue == " " {
		oldFieldValue = "отсутствует"
	}

	h.RemovePreviousMessage(b, &userId)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", menuHeader)+
			fmt.Sprintf("\n\nТекущее значение: <code>%s</code>", oldFieldValue)+
			fmt.Sprintf("\n\nВведи новое значение для поля <b>%s</b>:", fieldName),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
		})

	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to send message in handleCallbackEditField: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return handlers.NextConversationState(nextState)
}

// Handle publishing a profile to intro topic (with preview)
func (h *adminProfilesHandler) handlePublishCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return h.handlePublishProfile(b, ctx, false)
}

// Handle publishing a profile to intro topic without link preview
func (h *adminProfilesHandler) handlePublishNoPreviewCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return h.handlePublishProfile(b, ctx, true)
}

// Handle publishing a profile to intro topic
func (h *adminProfilesHandler) handlePublishProfile(b *gotgbot.Bot, ctx *ext.Context, withoutPreview bool) error {
	userId := ctx.EffectiveUser.Id
	msg := ctx.EffectiveMessage

	// Get user data
	userIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyUserID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: user ID not found in user store")
	}
	dbUserID := userIDVal.(int)

	// Get user from database
	dbUser, err := h.userRepository.GetByID(dbUserID)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to get user in handlePublishProfile: %w", err)
	}

	// Get profile from database
	profileIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyProfileID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: profile ID not found in user store")
	}
	profileID := profileIDVal.(int)

	profile, err := h.profileRepository.GetByID(profileID)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to get profile in handlePublishProfile: %w", err)
	}

	firstNameString := "└ ❌ Имя"
	lastNameString := "└ ❌ Фамилию"
	bioString := "└ ❌ Биографию"
	if dbUser != nil {
		if dbUser.Firstname != "" {
			firstNameString = "└ ✅ Имя"
		}
		if dbUser.Lastname != "" {
			lastNameString = "└ ✅ Фамилию"
		}
	}

	if profile != nil {
		if profile.Bio != "" {
			bioString = "└ ✅ Биографию"
		}
	}

	if !utils.IsProfileComplete(dbUser, profile) {
		h.RemovePreviousMessage(b, &userId)
		editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", adminProfilesMenuPublishHeader)+
				"\n\n⚠️ Профиль пользователя неполный. "+
				fmt.Sprintf("\n\nДля его публикации в канале \"<a href='%s'>Интро</a>\" необходимо указать: ",
					utils.GetIntroTopicLink(h.config))+
				"\n"+firstNameString+
				"\n"+lastNameString+
				"\n"+bioString,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
			})

		if err != nil {
			return fmt.Errorf("AdminProfilesHandler: failed to send message in handlePublishProfile: %w", err)
		}

		h.SavePreviousMessageInfo(userId, editedMsg)
		return nil // Stay in current state
	}

	// Format profile text for publishing
	publicMessageText := formatters.FormatPublicProfileForMessage(dbUser, profile, false)

	var publishedMsg *gotgbot.Message
	// Check if we need to update existing message or create a new one
	if profile.PublishedMessageID.Valid {
		// Try to edit existing message
		_, _, err := b.EditMessageText(
			publicMessageText,
			&gotgbot.EditMessageTextOpts{
				ChatId:    utils.ChatIdToFullChatId(h.config.SuperGroupChatID),
				MessageId: profile.PublishedMessageID.Int64,
				ParseMode: "HTML",
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: withoutPreview,
				},
			})
		// If editing fails, create a new message if the error is not about the message being exactly the same
		if err != nil && !strings.Contains(err.Error(), "are exactly the same") {
			publishedMsg, err = h.messageSenderService.SendHtmlWithReturnMessage(
				utils.ChatIdToFullChatId(h.config.SuperGroupChatID),
				publicMessageText,
				&gotgbot.SendMessageOpts{
					MessageThreadId: int64(h.config.IntroTopicID),
					LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
						IsDisabled: withoutPreview,
					},
				})
			if err != nil {
				return fmt.Errorf("AdminProfilesHandler: failed to publish profile: %w", err)
			}
		} else {
			// Message updated successfully, store the message ID for database update
			messageID := profile.PublishedMessageID.Int64
			publishedMsg = &gotgbot.Message{
				MessageId: messageID,
			}
		}
	} else {
		// Create a new message
		publishedMsg, err = h.messageSenderService.SendHtmlWithReturnMessage(
			utils.ChatIdToFullChatId(h.config.SuperGroupChatID),
			publicMessageText,
			&gotgbot.SendMessageOpts{
				MessageThreadId: int64(h.config.IntroTopicID),
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: withoutPreview,
				},
			})
		if err != nil {
			return fmt.Errorf("AdminProfilesHandler: failed to publish profile: %w", err)
		}
	}

	// Update profile with the published message ID
	err = h.profileRepository.UpdatePublishedMessageID(profile.ID, publishedMsg.MessageId)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to update published message ID: %w", err)
	}

	// Show success message
	h.RemovePreviousMessage(b, &userId)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", adminProfilesMenuPublishHeader)+
			fmt.Sprintf("\n\n✅ Профиль пользователя успешно опубликован в канале \"<a href='%s'>Интро</a>\"!", utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64)),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
		})

	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to send success message: %w", err)
	}

	h.SavePreviousMessageInfo(userId, editedMsg)
	return nil // Stay in current state
}

// Handle bio input
func (h *adminProfilesHandler) handleBioInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	bio := msg.Text
	userId := ctx.EffectiveUser.Id
	bioLength := utils.Utf16CodeUnitCount(bio)

	if bioLength > adminProfilesBioLengthLimit {
		h.RemovePreviousMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", adminProfilesMenuEditBioHeader)+
				fmt.Sprintf("\n\nТекущая длина: %d символов", bioLength)+
				fmt.Sprintf("\n\nПожалуйста, сократи до %d символов и пришли снова:", adminProfilesBioLengthLimit),
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
			})

		h.SavePreviousMessageInfo(userId, errMsg)
		return nil // Stay in current state
	}

	// Get profile ID from store
	profileIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyProfileID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: profile ID not found in user store")
	}
	profileID := profileIDVal.(int)

	// Save the bio
	err := h.profileRepository.Update(profileID, map[string]interface{}{
		"bio": bio,
	})
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to update bio: %w", err)
	}

	return h.returnToProfileView(b, ctx)
}

// Handle firstname input
func (h *adminProfilesHandler) handleFirstnameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	firstname := msg.Text
	userId := ctx.EffectiveUser.Id

	if len(firstname) > 30 {
		h.RemovePreviousMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", adminProfilesMenuEditFirstnameHeader)+
				"\n\nИмя слишком длинное. Пожалуйста, введи более короткое имя (не более 30 символов):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
			})

		h.SavePreviousMessageInfo(userId, errMsg)
		return nil // Stay in current state
	}

	// Get user ID from store
	userIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyUserID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: user ID not found in user store")
	}
	dbUserID := userIDVal.(int)

	// Save the firstname
	err := h.userRepository.Update(dbUserID, map[string]interface{}{
		"firstname": firstname,
	})
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to update firstname: %w", err)
	}

	return h.returnToProfileView(b, ctx)
}

// Handle lastname input
func (h *adminProfilesHandler) handleLastnameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	lastname := msg.Text
	userId := ctx.EffectiveUser.Id

	if len(lastname) > 30 {
		h.RemovePreviousMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", adminProfilesMenuEditLastnameHeader)+
				"\n\nФамилия слишком длинная. Пожалуйста, введи более короткую фамилию (не более 30 символов):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
			})

		h.SavePreviousMessageInfo(userId, errMsg)
		return nil // Stay in current state
	}

	// Get user ID from store
	userIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyUserID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: user ID not found in user store")
	}
	dbUserID := userIDVal.(int)

	// Save the lastname
	err := h.userRepository.Update(dbUserID, map[string]interface{}{
		"lastname": lastname,
	})
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to update lastname: %w", err)
	}

	return h.returnToProfileView(b, ctx)
}

// Handle coffee ban input
func (h *adminProfilesHandler) handleCoffeeBanInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	coffeeInput := strings.ToLower(strings.TrimSpace(msg.Text))
	userId := ctx.EffectiveUser.Id

	var coffeeValue bool
	if coffeeInput == "0" || coffeeInput == "нет" || coffeeInput == "разрешить" || coffeeInput == "разрешено" {
		coffeeValue = false
	} else if coffeeInput == "1" || coffeeInput == "да" || coffeeInput == "запретить" || coffeeInput == "запрещено" {
		coffeeValue = true
	} else {
		h.RemovePreviousMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", adminProfilesMenuCoffeeBanHeader)+
				"\n\nПожалуйста, введи корректное значение:"+
				"\n- 0, нет, разрешить, разрешено - чтобы разрешить кофейные встречи"+
				"\n- 1, да, запретить, запрещено - чтобы запретить кофейные встречи",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback),
			})

		h.SavePreviousMessageInfo(userId, errMsg)
		return nil // Stay in current state
	}

	// Get user ID from store
	userIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyUserID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: user ID not found in user store")
	}
	dbUserID := userIDVal.(int)

	// Update coffee ban status
	err := h.userRepository.SetCoffeeBan(dbUserID, coffeeValue)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to update coffee ban status: %w", err)
	}

	return h.returnToProfileView(b, ctx)
}

// Helper function to return to profile view after an update
func (h *adminProfilesHandler) returnToProfileView(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	// Clean input message
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	h.RemovePreviousMessage(b, &userId)

	// Get user data
	userIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyUserID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: user ID not found in user store")
	}
	dbUserID := userIDVal.(int)

	// Get user from database
	dbUser, err := h.userRepository.GetByID(dbUserID)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to get user in returnToProfileView: %w", err)
	}

	// Get profile from database
	profileIDVal, ok := h.userStore.Get(userId, adminProfilesCtxDataKeyProfileID)
	if !ok {
		return fmt.Errorf("AdminProfilesHandler: profile ID not found in user store")
	}
	profileID := profileIDVal.(int)

	profile, err := h.profileRepository.GetByID(profileID)
	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to get profile in returnToProfileView: %w", err)
	}

	successMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		"✅ Значение успешно сохранено!",
		nil)

	if err != nil {
		return fmt.Errorf("AdminProfilesHandler: failed to send success message: %w", err)
	}

	// Show updated profile after a brief delay
	time.Sleep(1 * time.Second)
	b.DeleteMessage(msg.Chat.Id, successMsg.MessageId, nil)

	// Show profile edit menu with updated data
	return h.showProfileEditMenu(b, msg, userId, dbUser, profile)
}

func (h *adminProfilesHandler) handleCancelCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	return h.handleCancel(b, ctx)
}

func (h *adminProfilesHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	_ = h.messageSenderService.Reply(msg, "Админ-сессия работы с профилями завершена.", nil)
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *adminProfilesHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			adminProfilesCtxDataKeyPreviousMessageID,
			adminProfilesCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *adminProfilesHandler) RemovePreviousMessage(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			adminProfilesCtxDataKeyPreviousMessageID,
			adminProfilesCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	b.DeleteMessage(chatID, messageID, nil)
}

func (h *adminProfilesHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		adminProfilesCtxDataKeyPreviousMessageID, adminProfilesCtxDataKeyPreviousChatID)
}
