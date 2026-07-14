package privatehandlers

import (
	"context"
	"database/sql"
	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/clients"
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
	profileStateViewOptions         = "profile_state_view_options"
	profileStateEditMyProfile       = "profile_state_edit_my_profile"
	profileStateAwaitQueryForSearch = "profile_state_await_query_for_search"
	profileStateAwaitBio            = "profile_state_await_bio"
	profileStateAwaitFirstname      = "profile_state_await_firstname"
	profileStateAwaitLastname       = "profile_state_await_lastname"

	// UserStore keys
	profileCtxDataKeyField                   = "profile_ctx_data_field"
	profileCtxDataKeyPreviousMessageID       = "profile_ctx_data_previous_message_id"
	profileCtxDataKeyPreviousChatID          = "profile_ctx_data_previous_chat_id"
	profileCtxDataKeyLastMessageTimeFromUser = "profile_ctx_data_last_message_time_from_user"
	profileCtxDataKeyCancelFunc              = "profile_ctx_data_key_cancel_func"

	// Menu headers
	profileMenuHeader              = "Меню \"Профиль\""
	profileMenuEditHeader          = "Профиль → Редактирование"
	profileMenuEditFirstnameHeader = "Профиль → Редактирование → Имя"
	profileMenuEditLastnameHeader  = "Профиль → Редактирование → Фамилия"
	profileMenuEditBioHeader       = "Профиль → Редактирование → О себе"
	profileMenuSearchHeader        = "Профиль → Поиск"
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
			handlers.NewCommand(constants.ProfileCommand, h.handleStart),
		},
		map[string][]ext.Handler{
			profileStateViewOptions: {
				handlers.NewCallback(callbackquery.Prefix(constants.ProfilePrefix), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
			},
			profileStateAwaitQueryForSearch: {
				handlers.NewMessage(message.Text, h.handleSearchProfileInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileFullCancel), h.handleCallbackCancel),
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

// Handles button clicks
func (h *profileHandler) handleCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	callback := ctx.Update.CallbackQuery
	data := callback.Data

	effectiveMsg := ctx.EffectiveMessage

	switch data {
	case constants.ProfileEditMyProfileCallback:
		return h.handleEditMyProfile(b, ctx, effectiveMsg)
	case constants.ProfileSearchProfileCallback:
		return h.handleSearchProfile(b, ctx, effectiveMsg)
	case constants.ProfileEditBioCallback:
		return h.handleEditField(b, ctx, effectiveMsg, fmt.Sprintf("обновлённую биографию (до %d символов)", constants.ProfileBioLengthLimit), profileStateAwaitBio)
	case constants.ProfileEditFirstnameCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новое имя", profileStateAwaitFirstname)
	case constants.ProfileEditLastnameCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новую фамилию", profileStateAwaitLastname)
	case constants.ProfileStartCallback:
		return h.handleStart(b, ctx)
	}

	return nil
}

// Entry point for the /profile command
func (h *profileHandler) handleStart(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser

	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.ProfileCommand) {
		return handlers.EndConversation()
	}

	h.RemovePreviousMessage(b, &user.Id)

	firstNameString := "└ ❌ Имя"
	lastNameString := "└ ❌ Фамилия"
	bioString := "└ ❌ Биография"
	profileLinkString := ""
	dbUser, err := h.userRepository.GetOrCreate(user)
	if err == nil {
		if dbUser.Firstname != "" {
			firstNameString = "└ ✅ Имя" + " <i>(" + dbUser.Firstname + ")</i>"
		}
		if dbUser.Lastname != "" {
			lastNameString = "└ ✅ Фамилия" + " <i>(" + dbUser.Lastname + ")</i>"
		}

		profile, err := h.profileRepository.GetOrCreate(dbUser.ID)
		if err == nil {
			if profile.PublishedMessageID.Valid {
				profileLinkString = fmt.Sprintf("👉 <a href='%s'>Ссылка</a> на твой профиль.",
					utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64))
			}
			if profile != nil {
				if profile.Bio != "" {
					bioString = "└ ✅ Биография"
				}
			}
		}
	}

	showProfileMenuText := fmt.Sprintf("<b>%s</b>", profileMenuHeader) +
		"\n\nТут ты можешь редактировать свой профиль и искать профили других пользователей по имени/нику." +
		fmt.Sprintf("\n\n<blockquote>⚠️ Профиль будет автоматически опубликован в канале \"<a href='%s'>Интро</a>\" как только все поля будут заполнены.</blockquote>",
			utils.GetIntroTopicLink(h.config)) +
		"\n\n" +
		"Статусы полей:" +
		"\n" +
		firstNameString +
		"\n" +
		lastNameString +
		"\n" +
		bioString +
		"\n\n" +
		profileLinkString

	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		showProfileMenuText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileMainButtons(),
		})

	if err != nil {
		return fmt.Errorf("%s: failed to send message in showProfileMenu: %w", utils.GetCurrentTypeName(), err)
	}

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

func (h *profileHandler) handleSearchProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
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

func (h *profileHandler) handleSearchProfileInput(b *gotgbot.Bot, ctx *ext.Context) error {
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
	if profile != nil && profile.PublishedMessageID.Valid {
		profileText += fmt.Sprintf("\n👉 <a href='%s'>Ссылка</a> на профиль в канале \"Интро\".",
			utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64))
	}
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

	// Try to publish profile if it is complete
	profilePublishedMessage, _ := h.tryToPublishProfile(b, ctx, true)

	h.RemovePreviousMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditBioHeader)+
			"\n\n✅ Биография сохранена!"+profilePublishedMessage,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
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

	// Try to publish profile if it is complete
	profilePublishedMessage, _ := h.tryToPublishProfile(b, ctx, true)

	// Send success message
	h.RemovePreviousMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditFirstnameHeader)+
			"\n\n✅ Имя сохранено!"+profilePublishedMessage,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
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

	// Try to publish profile if it is complete
	profilePublishedMessage, _ := h.tryToPublishProfile(b, ctx, true)

	// Send success message
	h.RemovePreviousMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditLastnameHeader)+
			"\n\n✅ Фамилия сохранена!"+profilePublishedMessage,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("%s: failed to send message in handleLastnameInput: %w", utils.GetCurrentTypeName(), err)
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
	userId := ctx.EffectiveUser.Id

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, profileCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Send(msg.Chat.Id, "Операция поиска профилей отменена.", nil)
		}
	} else {
		h.messageSenderService.Send(msg.Chat.Id, "Сессия работы с профилями завершена.", nil)
	}

	h.RemovePreviousMessage(b, &userId)
	h.userStore.Clear(userId)

	return handlers.EndConversation()
}

func (h *profileHandler) tryToPublishProfile(b *gotgbot.Bot, ctx *ext.Context, withoutPreview bool) (string, error) {
	dbUser, err := h.userRepository.GetOrCreate(ctx.EffectiveUser)
	if err != nil {
		return "", fmt.Errorf("%s: failed to get/create user in saveUserField: %w", utils.GetCurrentTypeName(), err)
	}

	profile, err := h.profileRepository.GetOrCreate(dbUser.ID)
	if err != nil {
		return "", fmt.Errorf("%s: failed to get/create profile in saveUserField: %w", utils.GetCurrentTypeName(), err)
	}

	// not saving profile if it is not complete
	if !h.profileService.IsProfileComplete(dbUser, profile) {
		return "", fmt.Errorf("%s: profile is not complete", utils.GetCurrentTypeName())
	}

	publicMessageText := formatters.FormatPublicProfileForMessage(dbUser, profile, false)

	var messageID int64
	if profile.PublishedMessageID.Valid {
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
		if err != nil && !strings.Contains(err.Error(), "are exactly the same") {
			msg, sendErr := h.messageSenderService.SendHtmlWithReturnMessage(
				utils.ChatIdToFullChatId(h.config.SuperGroupChatID),
				publicMessageText,
				&gotgbot.SendMessageOpts{
					MessageThreadId: int64(h.config.IntroTopicID),
					LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
						IsDisabled: withoutPreview,
					},
				})
			if sendErr != nil {
				return "", fmt.Errorf("%s: failed to publish profile: %w", utils.GetCurrentTypeName(), sendErr)
			}
			messageID = msg.MessageId
		} else {
			messageID = profile.PublishedMessageID.Int64
		}
	} else {
		msg, err := h.messageSenderService.SendHtmlWithReturnMessage(
			utils.ChatIdToFullChatId(h.config.SuperGroupChatID),
			publicMessageText,
			&gotgbot.SendMessageOpts{
				MessageThreadId: int64(h.config.IntroTopicID),
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: withoutPreview,
				},
			})
		if err != nil {
			return "", fmt.Errorf("%s: failed to publish profile: %w", utils.GetCurrentTypeName(), err)
		}
		messageID = msg.MessageId
	}

	if err := h.profileRepository.UpdatePublishedMessageID(profile.ID, messageID); err != nil {
		return "", fmt.Errorf("%s: failed to update published message ID: %w", utils.GetCurrentTypeName(), err)
	}

	profilePublishedMessage := fmt.Sprintf(
		"\n✅ Профиль <a href='%s'>опубликован</a> на канале \"Интро\".",
		utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64))
	return profilePublishedMessage, nil
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

func (h *profileHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		profileCtxDataKeyPreviousMessageID, profileCtxDataKeyPreviousChatID)
}
