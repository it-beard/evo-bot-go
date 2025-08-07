package privatehandlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"os"
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
	profileStateViewOptions               = "profile_state_view_options"
	profileStateEditMyProfile             = "profile_state_edit_my_profile"
	profileStateAwaitQueryForSearch       = "profile_state_await_query_for_search"
	profileStateAwaitQueryForBioSearch    = "profile_state_await_query_for_bio_search"
	profileStateAwaitBio                  = "profile_state_await_bio"
	profileStateAwaitFirstname            = "profile_state_await_firstname"
	profileStateAwaitLastname             = "profile_state_await_lastname"

	// UserStore keys
	profileCtxDataKeyField                   = "profile_ctx_data_field"
	profileCtxDataKeyPreviousMessageID       = "profile_ctx_data_previous_message_id"
	profileCtxDataKeyPreviousChatID          = "profile_ctx_data_previous_chat_id"
	profileCtxDataKeyLastMessageTimeFromUser = "profile_ctx_data_last_message_time_from_user"
	profileCtxDataKeyProcessing              = "profile_ctx_data_key_processing"
	profileCtxDataKeyCancelFunc              = "profile_ctx_data_key_cancel_func"

	// Menu headers
	profileMenuHeader               = "Меню \"Профиль\""
	profileMenuMyProfileHeader      = "Профиль → Мой профиль"
	profileMenuEditHeader           = "Профиль → Редактирование"
	profileMenuEditFirstnameHeader  = "Профиль → Редактирование → Имя"
	profileMenuEditLastnameHeader   = "Профиль → Редактирование → Фамилия"
	profileMenuEditBioHeader        = "Профиль → Редактирование → О себе"
	profileMenuPublishHeader        = "Профиль → Публикация"
	profileMenuSearchHeader         = "Профиль → Поиск"
	profileMenuBioSearchHeader      = "Профиль → Поиск по биографиям"

	// Callback data
	profileCallbackConfirmCancel = "profile_callback_confirm_cancel"
)

type profileHandler struct {
	config                      *config.Config
	messageSenderService        *services.MessageSenderService
	permissionsService          *services.PermissionsService
	profileService              *services.ProfileService
	userRepository              *repositories.UserRepository
	profileRepository           *repositories.ProfileRepository
	promptingTemplateRepository *repositories.PromptingTemplateRepository
	openaiClient                *clients.OpenAiClient
	userStore                   *utils.UserDataStore
}

func NewProfileHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
	profileService *services.ProfileService,
	userRepository *repositories.UserRepository,
	profileRepository *repositories.ProfileRepository,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
	openaiClient *clients.OpenAiClient,
) ext.Handler {
	h := &profileHandler{
		config:                      config,
		messageSenderService:        messageSenderService,
		permissionsService:          permissionsService,
		profileService:              profileService,
		userRepository:              userRepository,
		profileRepository:           profileRepository,
		promptingTemplateRepository: promptingTemplateRepository,
		openaiClient:                openaiClient,
		userStore:                   utils.NewUserDataStore(),
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
				handlers.NewCallback(callbackquery.Equal(constants.ProfilePublishCallback), h.handleCallback),
			},
			profileStateAwaitQueryForSearch: {
				handlers.NewMessage(message.Text, h.handleSearchInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitQueryForBioSearch: {
				handlers.NewMessage(message.Text, h.handleBioSearchInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(profileCallbackConfirmCancel), h.handleCallbackCancel),
			},
			profileStateAwaitBio: {
				handlers.NewMessage(message.Text, h.handleBioInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitFirstname: {
				handlers.NewMessage(message.Text, h.handleFirstnameInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitLastname: {
				handlers.NewMessage(message.Text, h.handleLastnameInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileEditMyProfileCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
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

func (h *profileHandler) showProfileMenu(b *gotgbot.Bot, msg *gotgbot.Message, userId int64) error {
	h.RemovePreviousMessage(b, &userId)

	profileTextAdditional := ""
	dbUser, err := h.userRepository.GetByTelegramID(userId)
	if err == nil {

		profile, err := h.profileRepository.GetOrCreate(dbUser.ID)
		if err == nil {
			if profile.PublishedMessageID.Valid {
				profileTextAdditional = fmt.Sprintf("\n\n👉 <a href='%s'>Ссылка</a> на твой профиль на канале \"Интро\".",
					utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64))
			}
		}
	}

	profileText := fmt.Sprintf("<b>%s</b>", profileMenuHeader) +
		fmt.Sprintf("\n\nТут ты можешь просматривать и редактировать свой профиль, публиковать его на канал \"<a href='%s'>Интро</a>\" и просматривать профили других пользователей.",
			utils.GetIntroTopicLink(h.config)) +
		profileTextAdditional

	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		profileText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileMainButtons(),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in showProfileMenu: %w", utils.GetCurrentTypeName(), err)
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

	switch data {
	case constants.ProfileViewMyProfileCallback:
		return h.handleViewMyProfile(b, ctx, effectiveMsg)
	case constants.ProfileEditMyProfileCallback:
		return h.handleEditMyProfile(b, ctx, effectiveMsg)
	case constants.ProfileViewOtherProfileCallback:
		return h.handleViewOtherProfile(b, ctx, effectiveMsg)
	case constants.ProfileBioSearchCallback:
		return h.handleBioSearch(b, ctx, effectiveMsg)
	case constants.ProfileEditBioCallback:
		return h.handleEditField(b, ctx, effectiveMsg, fmt.Sprintf("обновлённую биографию (до %d символов)", constants.ProfileBioLengthLimit), profileStateAwaitBio)
	case constants.ProfileEditFirstnameCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новое имя", profileStateAwaitFirstname)
	case constants.ProfileEditLastnameCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новую фамилию", profileStateAwaitLastname)
	case constants.ProfilePublishCallback:
		return h.handlePublishProfile(b, ctx, effectiveMsg, false)
	case constants.ProfilePublishWithoutPreviewCallback:
		return h.handlePublishProfile(b, ctx, effectiveMsg, true)
	case constants.ProfileStartCallback:
		return h.showProfileMenu(b, effectiveMsg, userId)
	}

	return nil
}

func (h *profileHandler) handleViewMyProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	// Get or create user in our DB
	user := ctx.Update.CallbackQuery.From
	dbUser, err := h.userRepository.GetOrCreate(&user)
	if err != nil {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении информации о пользователе.", nil)
		return fmt.Errorf("%s: failed to get user in handleViewMyProfile: %w", utils.GetCurrentTypeName(), err)
	}

	// Try to get profile
	profile, err := h.profileRepository.GetOrCreate(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении профиля.", nil)
		return fmt.Errorf("%s: failed to get profile in handleViewMyProfile: %w", utils.GetCurrentTypeName(), err)
	}

	profileText := fmt.Sprintf("<b>%s</b>\n\n%s", profileMenuMyProfileHeader, formatters.FormatProfileView(dbUser, profile, true))
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(msg.Chat.Id, profileText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileEditBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleViewMyProfile: %w", utils.GetCurrentTypeName(), err)
	}

	h.RemovePreviousMessage(b, &user.Id)
	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

func (h *profileHandler) handleEditMyProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	currentUser := ctx.Update.CallbackQuery.From

	h.RemovePreviousMessage(b, &currentUser.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuEditHeader)+
			"\n\nВыбери, что бы ты хотел/а изменить:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileEditButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleEditMyProfile: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(currentUser.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

func (h *profileHandler) handleViewOtherProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	user := ctx.Update.CallbackQuery.From

	h.RemovePreviousMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuSearchHeader)+
			"\n\nВведи телеграм-ник пользователя <i>(с @ или без)</i>, либо его имя и фамилию <i>(через пробел)</i>:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleViewOtherProfile: %w", utils.GetCurrentTypeName(), err)
	}
	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateAwaitQueryForSearch)
}

func (h *profileHandler) handleSearchInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	username := msg.Text
	userId := ctx.EffectiveMessage.From.Id

	// Remove @ if present
	username = strings.TrimPrefix(username, "@")

	dbUser, err := h.userRepository.GetByTelegramUsername(username)
	if err != nil && err == sql.ErrNoRows {
		//try to get user by full name
		parts := strings.Fields(username)
		firstname := parts[0]
		lastname := strings.Join(parts[1:], " ")

		dbUser, err = h.userRepository.SearchByName(firstname, lastname)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("%s: failed to search user in handleUsernameInput by full name: %w", utils.GetCurrentTypeName(), err)
		}
	} else if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("%s: failed to get user in handleUsernameInput by username: %w", utils.GetCurrentTypeName(), err)
	}

	// If user not found, show search again
	if err == sql.ErrNoRows {
		h.RemovePreviousMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuSearchHeader)+
				fmt.Sprintf("\n\nПользователь *%s* не найден.", username)+
				"\n\nПопробуй ещё раз, прислав мне имя пользователя снова:",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileStartCallback),
			})
		if err != nil {
			return fmt.Errorf("%s: failed to send message in handleUsernameInput: %w", utils.GetCurrentTypeName(), err)
		}

		h.SavePreviousMessageInfo(userId, editedMsg)
		return nil
	}

	// Try to get profile
	profile, err := h.profileRepository.GetOrCreate(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении профиля.", nil)
		return fmt.Errorf("%s: failed to get profile in handleUsernameInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.RemovePreviousMessage(b, &userId)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	profileText := fmt.Sprintf("<b>%s</b>\n\n%s", profileMenuSearchHeader, formatters.FormatProfileView(dbUser, profile, false))
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		profileText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleUsernameInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(ctx.EffectiveMessage.From.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

// Starts the process of editing a specific profile field
func (h *profileHandler) handleEditField(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message, fieldName string, nextState string) error {
	user := ctx.Update.CallbackQuery.From
	oldFieldValue := ""
	menuHeader := profileMenuEditHeader

	h.userStore.Set(user.Id, profileCtxDataKeyField, fieldName)

	dbUser, err := h.userRepository.GetOrCreate(&user)
	if err != nil {
		return fmt.Errorf("%s: failed to get user in handleEditField: %w", utils.GetCurrentTypeName(), err)
	}

	dbProfile, err := h.profileRepository.GetOrCreate(dbUser.ID)
	if err != nil {
		return fmt.Errorf("%s: failed to get/create profile in handleEditField: %w", utils.GetCurrentTypeName(), err)
	}

	switch nextState {
	case profileStateAwaitBio:
		dbProfile.Bio = strings.ReplaceAll(dbProfile.Bio, "<", "&lt;")
		dbProfile.Bio = strings.ReplaceAll(dbProfile.Bio, ">", "&gt;")
		oldFieldValue = "Текущее значение: <pre>" + dbProfile.Bio + "</pre>"
		menuHeader = profileMenuEditBioHeader
	case profileStateAwaitFirstname:
		oldFieldValue = "Текущее значение: <code>" + dbUser.Firstname + "</code>"
		menuHeader = profileMenuEditFirstnameHeader
	case profileStateAwaitLastname:
		oldFieldValue = "Текущее значение: <code>" + dbUser.Lastname + "</code>"
		menuHeader = profileMenuEditLastnameHeader
	}

	if oldFieldValue == "" || oldFieldValue == " " {
		oldFieldValue = "отсутствует"
	}

	h.RemovePreviousMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", menuHeader)+
			fmt.Sprintf("\n\n%s", oldFieldValue)+
			fmt.Sprintf("\n\nВведи %s:", fieldName),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleEditField: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(nextState)
}

// Bio handler
func (h *profileHandler) handleBioInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	bio := msg.Text
	bioLength := utils.Utf16CodeUnitCount(bio)

	// skip if it is sequential message from user with the same date
	lastMessageDate, ok := h.userStore.Get(msg.From.Id, profileCtxDataKeyLastMessageTimeFromUser)
	if ok && lastMessageDate == msg.Date {
		// Skip processing - same message date detected
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		return nil
	}

	// Store current message date to avoid duplicate processing
	h.userStore.Set(msg.From.Id, profileCtxDataKeyLastMessageTimeFromUser, msg.Date)

	if bioLength > constants.ProfileBioLengthLimit {
		h.RemovePreviousMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuEditBioHeader)+
				fmt.Sprintf("\n\nТекущая длина: %d символов", bioLength)+
				fmt.Sprintf("\n\nПожалуйста, сократи до %d символов и пришли снова:", constants.ProfileBioLengthLimit),
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "bio", bio)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditBioHeader)+
				"\n\nПроизошла ошибка при сохранении биографии.", nil)
		return fmt.Errorf("%s: failed to save bio in handleBioInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.RemovePreviousMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditBioHeader)+
			"\n\n✅ Биография сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleBioInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// Firstname handler
func (h *profileHandler) handleFirstnameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	firstname := msg.Text

	if len(firstname) > 30 {
		h.RemovePreviousMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuEditFirstnameHeader)+
				"\n\nИмя слишком длинное. Пожалуйста, введи более короткое имя (не более 30 символов):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveUserField(ctx.EffectiveUser, "firstname", firstname)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditFirstnameHeader)+
				"\n\nПроизошла ошибка при сохранении имени.", nil)
		return fmt.Errorf("%s: failed to save firstname in handleFirstnameInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.RemovePreviousMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditFirstnameHeader)+
			"\n\n✅ Имя сохранено!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleFirstnameInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// Lastname handler
func (h *profileHandler) handleLastnameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	lastname := msg.Text

	if len(lastname) > 30 {
		h.RemovePreviousMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuEditLastnameHeader)+
				"\n\nФамилия слишком длинная. Пожалуйста, введи более короткую фамилию (не более 30 символов):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveUserField(ctx.EffectiveUser, "lastname", lastname)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditLastnameHeader)+
				"\n\nПроизошла ошибка при сохранении фамилии.", nil)
		return fmt.Errorf("%s: failed to save lastname in handleLastnameInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.RemovePreviousMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditLastnameHeader)+
			"\n\n✅ Фамилия сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleLastnameInput: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// handlePublishProfile publishes the user's profile to the intro topic
func (h *profileHandler) handlePublishProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message, withoutPreview bool) error {
	user := ctx.Update.CallbackQuery.From
	dbUser, err := h.userRepository.GetOrCreate(&user)
	if err != nil {
		return fmt.Errorf("%s: failed to get user in handlePublishProfile: %w", utils.GetCurrentTypeName(), err)
	}

	profile, err := h.profileRepository.GetOrCreate(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("%s: failed to get profile in handlePublishProfile: %w", utils.GetCurrentTypeName(), err)
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

	if !h.profileService.IsProfileComplete(dbUser, profile) {
		h.RemovePreviousMessage(b, &user.Id)
		editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", profileMenuPublishHeader)+
				"\n\n⚠️ Твой профиль неполный. "+
				fmt.Sprintf("\n\nДля его публикации в канале \"<a href='%s'>Интро</a>\" необходимо указать: ", utils.GetIntroTopicLink(h.config))+
				"\n"+firstNameString+
				"\n"+lastNameString+
				"\n"+bioString,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.ProfileEditBackCancelButtons(constants.ProfileStartCallback),
			})

		if err != nil {
			return fmt.Errorf("%s: failed to send message in handlePublishProfile: %w", utils.GetCurrentTypeName(), err)
		}

		h.SavePreviousMessageInfo(user.Id, editedMsg)
		return handlers.NextConversationState(profileStateViewOptions)
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
				return fmt.Errorf("%s: failed to publish profile: %w", utils.GetCurrentTypeName(), err)
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
			return fmt.Errorf("%s: failed to publish profile: %w", utils.GetCurrentTypeName(), err)
		}
	}

	// Update profile with the published message ID
	err = h.profileRepository.UpdatePublishedMessageID(profile.ID, publishedMsg.MessageId)
	if err != nil {
		return fmt.Errorf("%s: failed to update published message ID: %w", utils.GetCurrentTypeName(), err)
	}

	// Show success message
	h.RemovePreviousMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuPublishHeader)+
			fmt.Sprintf("\n\n✅ Твой профиль успешно опубликован в канале \"<a href='%s'>Интро</a>\"!", utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64)),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send success message: %w", utils.GetCurrentTypeName(), err)
	}

	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

func (h *profileHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

func (h *profileHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, profileCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(msg, "Операция поиска профилей отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(msg, "Сессия работы с профилями завершена.", nil)
	}

	h.RemovePreviousMessage(b, &userId)
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *profileHandler) saveProfileField(tgUser *gotgbot.User, fieldName string, value string) error {
	dbUser, err := h.userRepository.GetOrCreate(tgUser)
	if err != nil {
		return fmt.Errorf("%s: failed to get/create user in saveProfileField: %w", utils.GetCurrentTypeName(), err)
	}

	// Try to get profile
	profile, err := h.profileRepository.GetOrCreate(dbUser.ID)
	if err != nil {
		return fmt.Errorf("%s: failed to get/create profile in saveProfileField: %w", utils.GetCurrentTypeName(), err)
	}

	// Profile exists, update the specific field
	fields := map[string]interface{}{
		fieldName: value,
	}

	err = h.profileRepository.Update(profile.ID, fields)
	if err != nil {
		return fmt.Errorf("%s: failed to update profile in saveProfileField: %w", utils.GetCurrentTypeName(), err)
	}

	return nil
}

func (h *profileHandler) saveUserField(tgUser *gotgbot.User, fieldName string, value string) error {
	dbUser, err := h.userRepository.GetOrCreate(tgUser)
	if err != nil {
		return fmt.Errorf("%s: failed to get/create user in saveUserField: %w", utils.GetCurrentTypeName(), err)
	}

	_, err = h.profileRepository.GetOrCreate(dbUser.ID)
	if err != nil {
		return fmt.Errorf("%s: failed to get/create profile in saveUserField: %w", utils.GetCurrentTypeName(), err)
	}

	// Update user with new field value
	fields := map[string]interface{}{
		fieldName: value,
	}

	err = h.userRepository.Update(dbUser.ID, fields)
	if err != nil {
		return fmt.Errorf("%s: failed to update user in saveUserField: %w", utils.GetCurrentTypeName(), err)
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

func (h *profileHandler) RemovePreviousMessage(b *gotgbot.Bot, userID *int64) {
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

func (h *profileHandler) handleBioSearch(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	user := ctx.Update.CallbackQuery.From

	h.RemovePreviousMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuBioSearchHeader)+
			"\n\nВведи поисковый запрос для поиска участников по их биографиям:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(profileCallbackConfirmCancel),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleBioSearch: %w", utils.GetCurrentTypeName(), err)
	}
	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateAwaitQueryForBioSearch)
}

func (h *profileHandler) handleBioSearchInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if we're already processing a request for this user
	if isProcessing, ok := h.userStore.Get(ctx.EffectiveUser.Id, profileCtxDataKeyProcessing); ok && isProcessing.(bool) {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, дождитесь окончания обработки предыдущего запроса, или используйте /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get query from user message
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Поисковый запрос не может быть пустым. Пожалуйста, введите запрос или используйте /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Mark as processing
	h.userStore.Set(ctx.EffectiveUser.Id, profileCtxDataKeyProcessing, true)

	// Create a cancellable context for this operation
	typingCtx, cancelTyping := context.WithCancel(context.Background())

	// Store cancel function in user store so it can be called from handleCancel
	h.userStore.Set(ctx.EffectiveUser.Id, profileCtxDataKeyCancelFunc, cancelTyping)

	// Make sure we clean up the processing flag in all exit paths
	defer func() {
		h.userStore.Set(ctx.EffectiveUser.Id, profileCtxDataKeyProcessing, false)
		h.userStore.Set(ctx.EffectiveUser.Id, profileCtxDataKeyCancelFunc, nil)
	}()

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Inform user that search has started
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(msg, fmt.Sprintf("Ищу участников по запросу: \"%s\"...", query), &gotgbot.SendMessageOpts{
		ReplyMarkup: buttons.CancelButton(profileCallbackConfirmCancel),
	})
	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)

	// Send typing action using MessageSender
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get profiles from database
	profiles, err := h.profileRepository.GetAllWithUsers()
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении профилей из базы данных.", nil)
		log.Printf("%s: Error during profiles retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	dataProfiles, err := h.prepareProfilesData(profiles)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при подготовке профилей для поиска.", nil)
		log.Printf("%s: Error during profiles preparation: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	templateText, err := h.promptingTemplateRepository.Get(prompts.GetProfilePromptTemplateDbKey)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении шаблона для поиска профилей.", nil)
		log.Printf("%s: Error during template retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	prompt := fmt.Sprintf(
		templateText,
		utils.EscapeMarkdown(string(dataProfiles)),
		utils.EscapeMarkdown(query),
	)

	// Save the prompt into a temporary file for logging purposes
	err = os.WriteFile("last-profile-prompt-log.txt", []byte(prompt), 0644)
	if err != nil {
		log.Printf("%s: Error writing prompt to file: %v", utils.GetCurrentTypeName(), err)
	}

	// Start periodic typing action every 5 seconds while waiting for the OpenAI response
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

	// Get completion from OpenAI using the new context
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

	err = h.messageSenderService.ReplyMarkdown(msg, responseOpenAi, nil)
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

func (h *profileHandler) prepareProfilesData(profiles []repositories.ProfileWithUser) ([]byte, error) {
	type ProfileData struct {
		ID          int    `json:"id"`
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		Username    string `json:"username,omitempty"`
		Bio         string `json:"bio"`
	}

	profileObjects := make([]ProfileData, 0, len(profiles))
	for _, profileWithUser := range profiles {
		profileObjects = append(profileObjects, ProfileData{
			ID:        profileWithUser.Profile.ID,
			FirstName: profileWithUser.User.Firstname,
			LastName:  profileWithUser.User.Lastname,
			Username:  profileWithUser.User.TgUsername,
			Bio:       profileWithUser.Profile.Bio,
		})
	}

	if len(profileObjects) == 0 {
		return nil, fmt.Errorf("%s: no profiles found in database", utils.GetCurrentTypeName())
	}

	dataProfiles, err := json.Marshal(profileObjects)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal profiles to JSON: %w", utils.GetCurrentTypeName(), err)
	}

	if string(dataProfiles) == "" {
		return nil, fmt.Errorf("%s: no profiles found in database", utils.GetCurrentTypeName())
	}

	return dataProfiles, nil
}

func (h *profileHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		profileCtxDataKeyPreviousMessageID, profileCtxDataKeyPreviousChatID)
}
