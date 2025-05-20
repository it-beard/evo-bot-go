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
	profileStateViewOptions            = "profile_state_view_options"
	profileStateEditMyProfile          = "profile_state_edit_my_profile"
	profileStateAwaitUsernameForSearch = "profile_state_await_username_for_search"
	profileStateAwaitBio               = "profile_state_await_bio"
	profileStateAwaitLinkedin          = "profile_state_await_linkedin"
	profileStateAwaitGithub            = "profile_state_await_github"
	profileStateAwaitFreeLink          = "profile_state_await_free_link"
	profileStateAwaitFirstname         = "profile_state_await_firstname"
	profileStateAwaitLastname          = "profile_state_await_lastname"

	// UserStore keys
	profileCtxDataKeyField             = "profile_ctx_data_field"
	profileCtxDataKeyPreviousMessageID = "profile_ctx_data_previous_message_id"
	profileCtxDataKeyPreviousChatID    = "profile_ctx_data_previous_chat_id"

	// Menu headers
	profileMenuHeader              = "Меню \"Профиль\""
	profileMenuMyProfileHeader     = "Профиль → Мой профиль"
	profileMenuEditHeader          = "Профиль → Редактирование"
	profileMenuEditFirstnameHeader = "Профиль → Редактирование → Имя"
	profileMenuEditLastnameHeader  = "Профиль → Редактирование → Фамилия"
	profileMenuEditBioHeader       = "Профиль → Редактирование → О себе"
	profileMenuEditLinkedinHeader  = "Профиль → Редактирование → LinkedIn"
	profileMenuEditGithubHeader    = "Профиль → Редактирование → GitHub"
	profileMenuEditFreeLinkHeader  = "Профиль → Редактирование → Ссылка"
	profileMenuPublishHeader       = "Профиль → Публикация"
	profileMenuSearchHeader        = "Профиль → Поиск"
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
				handlers.NewCallback(callbackquery.Equal(constants.ProfilePublishCallback), h.handleCallback),
			},
			profileStateAwaitUsernameForSearch: {
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
			profileStateAwaitFreeLink: {
				handlers.NewMessage(message.Text, h.handleFreeLinkInput),
				handlers.NewCallback(callbackquery.Equal(constants.ProfileStartCallback), h.handleCallback),
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
	h.RemovePreviouseMessage(b, &userId)

	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuHeader)+
			fmt.Sprintf("\n\nТут ты можешь просматривать и редактировать свой профиль, публиковать его на канал \"<a href='%s'>Интро</a>\" и просматривать профили других пользователей.",
				utils.GetIntroTopicLink(h.config)),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileMainButtons(),
		})

	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in showProfileMenu: %w", err)
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
	case constants.ProfileEditBioCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "обновлённую биографию (до 2000 символов)", profileStateAwaitBio)
	case constants.ProfileEditLinkedinCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новую ссылку на LinkedIn", profileStateAwaitLinkedin)
	case constants.ProfileEditGithubCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новую ссылку на GitHub", profileStateAwaitGithub)
	case constants.ProfileEditFreeLinkCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новую произвольную ссылку", profileStateAwaitFreeLink)
	case constants.ProfileEditFirstnameCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новое имя", profileStateAwaitFirstname)
	case constants.ProfileEditLastnameCallback:
		return h.handleEditField(b, ctx, effectiveMsg, "новую фамилию", profileStateAwaitLastname)
	case constants.ProfilePublishCallback:
		return h.handlePublishProfile(b, ctx, effectiveMsg)
	case constants.ProfileStartCallback:
		return h.showProfileMenu(b, effectiveMsg, userId)
	}

	return nil
}

func (h *profileHandler) handleViewMyProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	// Get or create user in our DB
	user := ctx.Update.CallbackQuery.From
	dbUser, err := h.userRepository.GetOrCreateUser(&user)
	if err != nil {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении информации о пользователе.", nil)
		return fmt.Errorf("ProfileHandler: failed to get user in handleViewMyProfile: %w", err)
	}

	// Try to get profile
	profile, err := h.profileRepository.GetByUserID(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении профиля.", nil)
		return fmt.Errorf("ProfileHandler: failed to get profile in handleViewMyProfile: %w", err)
	}

	profileText := fmt.Sprintf("<b>%s</b>\n\n%s", profileMenuMyProfileHeader, formatters.FormatProfileView(dbUser, profile, true))
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(msg.Chat.Id, profileText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileEditBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleViewMyProfile: %w", err)
	}

	h.RemovePreviouseMessage(b, &user.Id)
	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

func (h *profileHandler) handleEditMyProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	currentUser := ctx.Update.CallbackQuery.From

	h.RemovePreviouseMessage(b, &currentUser.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuEditHeader)+
			"\n\nВыбери, что бы ты хотел/а изменить:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileEditButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleEditMyProfile: %w", err)
	}

	h.SavePreviousMessageInfo(currentUser.Id, editedMsg)
	return handlers.NextConversationState(profileStateViewOptions)
}

func (h *profileHandler) handleViewOtherProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	user := ctx.Update.CallbackQuery.From

	h.RemovePreviouseMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuSearchHeader)+
			"\n\nВведи имя пользователя (с @ или без):",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleViewOtherProfile: %w", err)
	}
	h.SavePreviousMessageInfo(user.Id, editedMsg)
	return handlers.NextConversationState(profileStateAwaitUsernameForSearch)
}

func (h *profileHandler) handleUsernameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	username := msg.Text
	userId := ctx.EffectiveMessage.From.Id

	// Remove @ if present
	username = strings.TrimPrefix(username, "@")

	dbUser, err := h.userRepository.GetByTelegramUsername(username)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении информации о пользователе.", nil)
		return fmt.Errorf("ProfileHandler: failed to get user in handleUsernameInput: %w", err)
	}

	// If user not found, show search again
	if err == sql.ErrNoRows {
		h.RemovePreviouseMessage(b, &userId)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuSearchHeader)+
				fmt.Sprintf("\n\nПользователь *%s* не найден.", username)+
				"\n\nПопробуй ещё раз, прислав мне имя пользователя снова:",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileStartCallback),
			})
		if err != nil {
			return fmt.Errorf("ProfileHandler: failed to send message in handleUsernameInput: %w", err)
		}

		h.SavePreviousMessageInfo(userId, editedMsg)
		return nil
	}

	// Try to get profile
	profile, err := h.profileRepository.GetByUserID(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		_ = h.messageSenderService.Reply(msg,
			"Произошла ошибка при получении профиля.", nil)
		return fmt.Errorf("ProfileHandler: failed to get profile in handleUsernameInput: %w", err)
	}

	h.RemovePreviouseMessage(b, &userId)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	profileText := fmt.Sprintf("<b>%s</b>\n\n%s", profileMenuSearchHeader, formatters.FormatProfileView(dbUser, profile, false))
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		profileText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleUsernameInput: %w", err)
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

	dbUser, err := h.userRepository.GetOrCreateUser(&user)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to get user in handleEditField: %w", err)
	}

	dbProfile, err := h.profileRepository.GetOrCreateDefaultProfile(dbUser.ID)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to get/create profile in handleEditField: %w", err)
	}

	switch nextState {
	case profileStateAwaitBio:
		oldFieldValue = dbProfile.Bio
		menuHeader = profileMenuEditBioHeader
	case profileStateAwaitLinkedin:
		oldFieldValue = dbProfile.LinkedIn
		menuHeader = profileMenuEditLinkedinHeader
	case profileStateAwaitGithub:
		oldFieldValue = dbProfile.GitHub
		menuHeader = profileMenuEditGithubHeader
	case profileStateAwaitFreeLink:
		oldFieldValue = dbProfile.FreeLink
		menuHeader = profileMenuEditFreeLinkHeader
	case profileStateAwaitFirstname:
		oldFieldValue = dbUser.Firstname
		menuHeader = profileMenuEditFirstnameHeader
	case profileStateAwaitLastname:
		oldFieldValue = dbUser.Lastname
		menuHeader = profileMenuEditLastnameHeader
	}

	if oldFieldValue == "" || oldFieldValue == " " {
		oldFieldValue = "отсутствует"
	}

	h.RemovePreviouseMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("*%s*", menuHeader)+
			fmt.Sprintf("\n\nТекущее значение: `%s`", oldFieldValue)+
			fmt.Sprintf("\n\nВведи %s:", fieldName),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
		})

	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleEditField: %w", err)
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
			fmt.Sprintf("*%s*", profileMenuEditBioHeader)+
				"\n\nБиография слишком длинная. Пожалуйста, сократи до 2000 символов и пришли снова:",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "bio", bio)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditBioHeader)+
				"\n\nПроизошла ошибка при сохранении биографии.", nil)
		return fmt.Errorf("ProfileHandler: failed to save bio in handleBioInput: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditBioHeader)+
			"\n\n✅ Биография сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleBioInput: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// Firstname handler
func (h *profileHandler) handleFirstnameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	firstname := msg.Text

	if len(firstname) > 30 {
		h.RemovePreviouseMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuEditFirstnameHeader)+
				"\n\nИмя слишком длинное. Пожалуйста, введи более короткое имя (не более 30 символов):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveUserField(ctx.EffectiveUser, "firstname", firstname)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditFirstnameHeader)+
				"\n\nПроизошла ошибка при сохранении имени.", nil)
		return fmt.Errorf("ProfileHandler: failed to save firstname in handleFirstnameInput: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditFirstnameHeader)+
			"\n\n✅ Имя сохранено!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleFirstnameInput: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// Lastname handler
func (h *profileHandler) handleLastnameInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	lastname := msg.Text

	if len(lastname) > 30 {
		h.RemovePreviouseMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuEditLastnameHeader)+
				"\n\nФамилия слишком длинная. Пожалуйста, введи более короткую фамилию (не более 30 символов):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveUserField(ctx.EffectiveUser, "lastname", lastname)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditLastnameHeader)+
				"\n\nПроизошла ошибка при сохранении фамилии.", nil)
		return fmt.Errorf("ProfileHandler: failed to save lastname in handleLastnameInput: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditLastnameHeader)+
			"\n\n✅ Фамилия сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleLastnameInput: %w", err)
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
			fmt.Sprintf("*%s*", profileMenuEditLinkedinHeader)+
				"\n\nПожалуйста, введи корректную ссылку на LinkedIn (содержит \"linkedin\"):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "linkedin", linkedin)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditLinkedinHeader)+
				"\n\nПроизошла ошибка при сохранении ссылки на LinkedIn.", nil)
		return fmt.Errorf("ProfileHandler: failed to save LinkedIn in handleLinkedinInput: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditLinkedinHeader)+
			"\n\n✅ Ссылка на LinkedIn сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleLinkedinInput: %w", err)
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
			fmt.Sprintf("*%s*", profileMenuEditGithubHeader)+
				"\n\nПожалуйста, введи корректную ссылку на GitHub (содержит \"github\"):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "github", github)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditGithubHeader)+
				"\n\nПроизошла ошибка при сохранении ссылки на GitHub.", nil)
		return fmt.Errorf("ProfileHandler: failed to save GitHub in handleGithubInput: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditGithubHeader)+
			"\n\n✅ Ссылка на GitHub сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleGithubInput: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// Free link handler
func (h *profileHandler) handleFreeLinkInput(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	freeLink := msg.Text

	if !strings.Contains(freeLink, "https://") {
		h.RemovePreviouseMessage(b, &msg.From.Id)
		b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
		errMsg, _ := h.messageSenderService.SendMarkdownWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("*%s*", profileMenuEditFreeLinkHeader)+
				"\n\nПожалуйста, введи корректную ссылку (начинается с https://):",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileEditMyProfileCallback),
			})

		h.SavePreviousMessageInfo(msg.From.Id, errMsg)
		return nil
	}

	err := h.saveProfileField(ctx.EffectiveUser, "freeLink", freeLink)
	if err != nil {
		_ = h.messageSenderService.ReplyMarkdown(msg,
			fmt.Sprintf("*%s*", profileMenuEditFreeLinkHeader)+
				"\n\nПроизошла ошибка при сохранении произвольной ссылки.", nil)
		return fmt.Errorf("ProfileHandler: failed to save freeLink in handleFreeLinkInput: %w", err)
	}

	h.RemovePreviouseMessage(b, &msg.From.Id)
	b.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	sendMsg, err := h.messageSenderService.SendMarkdownWithReturnMessage(msg.Chat.Id,
		fmt.Sprintf("*%s*", profileMenuEditFreeLinkHeader)+
			"\n\n✅ Произвольная ссылка сохранена!",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackPublishCancelButtons(constants.ProfileEditMyProfileCallback),
		})
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send message in handleFreeLinkInput: %w", err)
	}

	h.SavePreviousMessageInfo(msg.From.Id, sendMsg)
	return handlers.NextConversationState(profileStateEditMyProfile)
}

// handlePublishProfile publishes the user's profile to the intro topic
func (h *profileHandler) handlePublishProfile(b *gotgbot.Bot, ctx *ext.Context, msg *gotgbot.Message) error {
	user := ctx.Update.CallbackQuery.From
	dbUser, err := h.userRepository.GetOrCreateUser(&user)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to get user in handlePublishProfile: %w", err)
	}

	firstNameString := "└ ❌ Имя"
	lastNameString := "└ ❌ Фамилию"
	if dbUser != nil {
		if dbUser.Firstname != "" {
			firstNameString = "└ ✅ Имя"
		}
		if dbUser.Lastname != "" {
			lastNameString = "└ ✅ Фамилию"
		}
	}

	profile, err := h.profileRepository.GetByUserID(dbUser.ID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("ProfileHandler: failed to get profile in handlePublishProfile: %w", err)
	}

	bioString := "└ ❌ Биографию"
	linkedInString := "└ ❌ LinkedIn"
	gitHubString := "└ ❌ GitHub"
	freeLinkString := "└ ❌ Ссылка"
	if profile != nil {
		if profile.Bio != "" {
			bioString = "└ ✅ Биографию"
		}
		if profile.LinkedIn != "" {
			linkedInString = "└ ✅ LinkedIn"
		}
		if profile.GitHub != "" {
			gitHubString = "└ ✅ GitHub"
		}
		if profile.FreeLink != "" {
			freeLinkString = "└ ✅ Ссылка"
		}
	}

	if !utils.IsProfileComplete(dbUser, profile) {
		h.RemovePreviouseMessage(b, &user.Id)
		editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("<b>%s</b>", profileMenuPublishHeader)+
				"\n\n⚠️ Твой профиль неполный. "+
				fmt.Sprintf("\n\nДля его публикации в канале \"<a href='%s'>Интро</a>\" необходимо указать: ", utils.GetIntroTopicLink(h.config))+
				"\n"+firstNameString+" "+
				"\n"+lastNameString+" "+
				"\n"+bioString+" "+
				"\n\nИ хотя бы одну ссылку: "+
				"\n"+linkedInString+" "+
				"\n"+gitHubString+" "+
				"\n"+freeLinkString+" ",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: formatters.ProfileEditBackCancelButtons(constants.ProfileStartCallback),
			})

		if err != nil {
			return fmt.Errorf("ProfileHandler: failed to send message in handlePublishProfile: %w", err)
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
					IsDisabled: false,
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
						IsDisabled: false,
					},
				})
			if err != nil {
				return fmt.Errorf("ProfileHandler: failed to publish profile: %w", err)
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
					IsDisabled: false,
				},
			})
		if err != nil {
			return fmt.Errorf("ProfileHandler: failed to publish profile: %w", err)
		}
	}

	// Update profile with the published message ID
	err = h.profileRepository.UpdatePublishedMessageID(profile.ID, publishedMsg.MessageId)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to update published message ID: %w", err)
	}

	// Show success message
	h.RemovePreviouseMessage(b, &user.Id)
	editedMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("<b>%s</b>", profileMenuPublishHeader)+
			fmt.Sprintf("\n\n✅ Твой профиль успешно опубликован в канале \"<a href='%s'>Интро</a>\"!", utils.GetIntroMessageLink(h.config, profile.PublishedMessageID.Int64)),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.ProfileBackCancelButtons(constants.ProfileStartCallback),
		})

	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to send success message: %w", err)
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

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	_ = h.messageSenderService.Reply(msg, "Сессия работы с профилями завершена.", nil)
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *profileHandler) saveProfileField(tgUser *gotgbot.User, fieldName string, value string) error {
	dbUser, err := h.userRepository.GetOrCreateUser(tgUser)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to get/create user in saveProfileField: %w", err)
	}

	// Try to get profile
	profile, err := h.profileRepository.GetOrCreateDefaultProfile(dbUser.ID)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to get/create profile in saveProfileField: %w", err)
	}

	// Profile exists, update the specific field
	fields := map[string]interface{}{
		fieldName: value,
	}

	err = h.profileRepository.Update(profile.ID, fields)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to update profile in saveProfileField: %w", err)
	}

	return nil
}

func (h *profileHandler) saveUserField(tgUser *gotgbot.User, fieldName string, value string) error {
	dbUser, err := h.userRepository.GetOrCreateUser(tgUser)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to get/create user in saveUserField: %w", err)
	}

	_, err = h.profileRepository.GetOrCreateDefaultProfile(dbUser.ID)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to get/create profile in saveUserField: %w", err)
	}

	// Update user with new field value
	fields := map[string]interface{}{
		fieldName: value,
	}

	err = h.userRepository.Update(dbUser.ID, fields)
	if err != nil {
		return fmt.Errorf("ProfileHandler: failed to update user in saveUserField: %w", err)
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
