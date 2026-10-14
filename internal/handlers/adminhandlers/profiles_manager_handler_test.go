package adminhandlers

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"
	"evo-bot-go/internal/utils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const profilesSessionEndedText = "Админ-сессия работы с профилями завершена."

var errDB = errors.New("db is down")

func sampleUser() repositories.User {
	return repositories.User{
		ID:           11,
		TgID:         777001,
		Firstname:    "Ivan",
		Lastname:     "Petrov",
		TgUsername:   "ivanp",
		Score:        42,
		IsClubMember: true,
	}
}

func sampleProfile() repositories.Profile {
	return repositories.Profile{ID: 21, UserID: 11, Bio: "Go developer"}
}

func profilesCommand() string {
	return "/" + constants.AdminProfilesCommand
}

// openMainMenu starts the conversation and returns the main menu message ID
func (e *testEnv) openMainMenu() int64 {
	e.t.Helper()
	e.mustText(profilesCommand())
	return e.lastSentID()
}

// openProfile brings the conversation to the profile edit menu and returns the menu message ID
func (e *testEnv) openProfile(u repositories.User, p repositories.Profile) int64 {
	e.t.Helper()
	e.openMainMenu()
	e.mustClick(constants.AdminProfilesSearchByTelegramIDCallback)
	e.mock.ExpectQuery(sqlUserByTgID).WithArgs(u.TgID).WillReturnRows(userRows(u))
	e.expectProfileByUserID(p)
	e.mustText(strconv.FormatInt(u.TgID, 10))
	e.client.Reset()
	return e.lastSentID()
}

// openField opens the profile edit menu, clicks an edit-field button and returns the prompt message ID
func (e *testEnv) openField(u repositories.User, p repositories.Profile, callback string) int64 {
	e.t.Helper()
	e.openProfile(u, p)
	e.expectUserByID(u)
	e.expectProfileByID(p)
	e.mustClick(callback)
	e.client.Reset()
	return e.lastSentID()
}

func assertEditMenu(t *testing.T, call testutils.APICall, u repositories.User) {
	t.Helper()
	assert.Equal(t, testAdminID, call.Int64("chat_id"))
	assert.Equal(t, "HTML", call.Params["parse_mode"])
	text := call.Params["text"]
	assert.True(t, strings.HasPrefix(text, "<b>"+adminProfilesMenuEditHeader+"</b>"), text)
	fullName := u.Firstname
	if u.Lastname != "" {
		fullName += " " + u.Lastname
	}
	assert.Contains(t, text, fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", u.TgID, fullName))
	assert.Contains(t, text, fmt.Sprintf("<i>Telegram ID:</i> <code>%d</code>", u.TgID))
	assert.Equal(t, buttons.ProfilesEditMenuButtons(constants.AdminProfilesStartCallback), markup(t, call))
}

func assertMainMenu(t *testing.T, call testutils.APICall) {
	t.Helper()
	assert.Equal(t, testAdminID, call.Int64("chat_id"))
	assert.Equal(t, "HTML", call.Params["parse_mode"])
	assert.True(t, strings.HasPrefix(call.Params["text"], "<b>"+adminProfilesMenuHeader+"</b>"), call.Params["text"])
	assert.Equal(t, buttons.ProfilesMainMenuButtons(), markup(t, call))
}

func TestAdminProfilesHandler_RejectsNonAdmin(t *testing.T) {
	e := newProfilesEnv(t)
	msg := testutils.NewPrivateMessage(testMemberID, profilesCommand())

	require.NoError(t, e.dispatch(testutils.MessageUpdate(msg)))

	memberCall, ok := e.client.LastCallTo("getChatMember")
	require.True(t, ok)
	assert.Equal(t, fullGroupChatID(), memberCall.Int64("chat_id"))
	assert.Equal(t, testMemberID, memberCall.Int64("user_id"))

	assert.Equal(t, []string{"Эта команда доступна только администраторам."}, e.client.SentTexts())
	var reply gotgbot.ReplyParameters
	require.NoError(t, e.lastSendMessage().DecodeJSON("reply_parameters", &reply))
	assert.Equal(t, msg.MessageId, reply.MessageId)

	// The conversation was not started
	e.notHandled(callbackUpdate(testMemberID, constants.AdminProfilesSearchByUsernameCallback))
}

func TestAdminProfilesHandler_RejectsGroupChat(t *testing.T) {
	e := newProfilesEnv(t)
	const groupChatID int64 = -100500
	msg := testutils.NewGroupMessage(testAdminID, groupChatID, 0, profilesCommand())

	require.NoError(t, e.dispatch(testutils.MessageUpdate(msg)))

	sends := e.client.CallsTo("sendMessage")
	require.Len(t, sends, 2)
	assert.Equal(t, groupChatID, sends[0].Int64("chat_id"))
	assert.Equal(t, "Markdown", sends[0].Params["parse_mode"])
	assert.Contains(t, sends[0].Params["text"], "Эта команда работает только в _личной беседе_ со мной")
	// The user gets pinged in private
	assert.Equal(t, testAdminID, sends[1].Int64("chat_id"))
	for _, text := range e.client.SentTexts() {
		assert.NotContains(t, text, adminProfilesMenuHeader)
	}

	e.notHandled(callbackUpdate(testAdminID, constants.AdminProfilesSearchByUsernameCallback))
}

func TestAdminProfilesHandler_CommandShowsMainMenu(t *testing.T) {
	e := newProfilesEnv(t)

	e.mustText(profilesCommand())

	call := e.lastSendMessage()
	assertMainMenu(t, call)
	assert.Contains(t, call.Params["text"], "Здесь ты можешь редактировать профили пользователей")
	assert.True(t, linkPreviewDisabled(t, call))
	assert.Empty(t, e.client.CallsTo("deleteMessage"))
}

func TestAdminProfilesHandler_MainMenuNavigation(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		header   string
		prompt   string
	}{
		{
			name:     "search by username",
			callback: constants.AdminProfilesSearchByUsernameCallback,
			header:   adminProfilesMenuHeader,
			prompt:   "Введи имя пользователя (с @ или без) для поиска:",
		},
		{
			name:     "search by telegram id",
			callback: constants.AdminProfilesSearchByTelegramIDCallback,
			header:   adminProfilesMenuSearchByIDHeader,
			prompt:   "Введи ID пользователя Telegram для поиска профиля:",
		},
		{
			name:     "search by full name",
			callback: constants.AdminProfilesSearchByFullNameCallback,
			header:   adminProfilesMenuSearchByNameHeader,
			prompt:   "Введи имя и фамилию пользователя (через пробел) для поиска профиля:",
		},
		{
			name:     "create by forwarded message",
			callback: constants.AdminProfilesCreateByForwardedMessageCallback,
			header:   adminProfilesMenuHeader,
			prompt:   "Перешли мне сообщение от пользователя, для которого нужно создать профиль:",
		},
		{
			name:     "create by telegram id",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			header:   adminProfilesMenuCreateByIDHeader,
			prompt:   "Введи ID пользователя Telegram для создания профиля:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			menuID := e.openMainMenu()
			e.client.Reset()

			// Open the sub-menu: the main menu is replaced with a prompt
			e.mustClick(tt.callback)
			assert.Equal(t, []int64{menuID}, e.deletedMessageIDs())
			prompt := e.lastSendMessage()
			assert.Equal(t, testAdminID, prompt.Int64("chat_id"))
			assert.Equal(t, "HTML", prompt.Params["parse_mode"])
			assert.Equal(t, "<b>"+tt.header+"</b>\n\n"+tt.prompt, prompt.Params["text"])
			assert.Equal(t, buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback), markup(t, prompt))
			promptID := e.lastSentID()

			// "Back" returns to the main menu
			e.client.Reset()
			e.mustClick(constants.AdminProfilesStartCallback)
			assert.Equal(t, []int64{promptID}, e.deletedMessageIDs())
			assertMainMenu(t, e.lastSendMessage())

			// "Cancel" from the prompt ends the session
			e.mustClick(tt.callback)
			promptID = e.lastSentID()
			e.client.Reset()
			e.mustClick(constants.AdminProfilesCancelCallback)
			assert.Equal(t, []int64{promptID}, e.deletedMessageIDs())
			assert.Equal(t, []string{profilesSessionEndedText}, e.client.SentTexts())
			e.notHandled(callbackUpdate(testAdminID, constants.AdminProfilesStartCallback))
		})
	}
}

func TestAdminProfilesHandler_CancelEndsSession(t *testing.T) {
	user, profile := sampleUser(), sampleProfile()

	tests := []struct {
		name   string
		open   func(e *testEnv) int64
		cancel func(e *testEnv)
	}{
		{
			name:   "cancel button in main menu",
			open:   func(e *testEnv) int64 { return e.openMainMenu() },
			cancel: func(e *testEnv) { e.mustClick(constants.AdminProfilesCancelCallback) },
		},
		{
			name:   "cancel command in main menu",
			open:   func(e *testEnv) int64 { return e.openMainMenu() },
			cancel: func(e *testEnv) { e.mustText("/" + constants.CancelCommand) },
		},
		{
			name:   "cancel button in profile edit menu",
			open:   func(e *testEnv) int64 { return e.openProfile(user, profile) },
			cancel: func(e *testEnv) { e.mustClick(constants.AdminProfilesCancelCallback) },
		},
		{
			name:   "cancel command in profile edit menu",
			open:   func(e *testEnv) int64 { return e.openProfile(user, profile) },
			cancel: func(e *testEnv) { e.mustText("/" + constants.CancelCommand) },
		},
		{
			name: "cancel button in field prompt",
			open: func(e *testEnv) int64 {
				return e.openField(user, profile, constants.AdminProfilesEditBioCallback)
			},
			cancel: func(e *testEnv) { e.mustClick(constants.AdminProfilesCancelCallback) },
		},
		{
			name: "cancel button in coffee ban menu",
			open: func(e *testEnv) int64 {
				return e.openField(user, profile, constants.AdminProfilesEditCoffeeBanCallback)
			},
			cancel: func(e *testEnv) { e.mustClick(constants.AdminProfilesCancelCallback) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			prevID := tt.open(e)
			e.client.Reset()

			tt.cancel(e)

			assert.Equal(t, []int64{prevID}, e.deletedMessageIDs())
			assert.Equal(t, []string{profilesSessionEndedText}, e.client.SentTexts())
			assert.Equal(t, testAdminID, e.lastSendMessage().Int64("chat_id"))
			e.notHandled(callbackUpdate(testAdminID, constants.AdminProfilesStartCallback))

			// A new session can be started and does not try to delete stale messages
			e.client.Reset()
			e.mustText(profilesCommand())
			assert.Empty(t, e.client.CallsTo("deleteMessage"))
			assertMainMenu(t, e.lastSendMessage())
		})
	}
}

func TestAdminProfilesHandler_FallbackDeletesUnexpectedText(t *testing.T) {
	e := newProfilesEnv(t)
	e.openMainMenu()
	e.client.Reset()

	msg := e.mustText("random text")

	deletes := e.client.CallsTo("deleteMessage")
	require.Len(t, deletes, 1)
	assert.Equal(t, testAdminID, deletes[0].Int64("chat_id"))
	assert.Equal(t, msg.MessageId, deletes[0].Int64("message_id"))
	assert.Empty(t, e.client.CallsTo("sendMessage"))

	// Still in the main menu state
	e.mustClick(constants.AdminProfilesSearchByUsernameCallback)
	assert.Contains(t, e.lastSendMessage().Params["text"], "Введи имя пользователя")
}

func TestAdminProfilesHandler_FindProfile(t *testing.T) {
	user, profile := sampleUser(), sampleProfile()
	newUser := repositories.User{ID: 11, TgID: 777001}

	tests := []struct {
		name     string
		callback string
		input    string
		setup    func(e *testEnv)
		// the user shown in the edit menu
		shown repositories.User
	}{
		{
			name:     "by username with @",
			callback: constants.AdminProfilesSearchByUsernameCallback,
			input:    "@ivanp",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByUsername).WithArgs("ivanp").WillReturnRows(userRows(user))
				e.expectProfileByUserID(profile)
			},
			shown: user,
		},
		{
			name:     "by username without @",
			callback: constants.AdminProfilesSearchByUsernameCallback,
			input:    "ivanp",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByUsername).WithArgs("ivanp").WillReturnRows(userRows(user))
				e.expectProfileByUserID(profile)
			},
			shown: user,
		},
		{
			name:     "by telegram id",
			callback: constants.AdminProfilesSearchByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WithArgs(int64(777001)).WillReturnRows(userRows(user))
				e.expectProfileByUserID(profile)
			},
			shown: user,
		},
		{
			name:     "by full name with extra spaces",
			callback: constants.AdminProfilesSearchByFullNameCallback,
			input:    "  Ivan   Petrov  ",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByName).WithArgs("Ivan", "Petrov").WillReturnRows(userRows(user))
				e.expectProfileByUserID(profile)
			},
			shown: user,
		},
		{
			name:     "by full name with compound last name",
			callback: constants.AdminProfilesSearchByFullNameCallback,
			input:    "Ivan Petrov Vodkin",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByName).WithArgs("Ivan", "Petrov Vodkin").WillReturnRows(userRows(user))
				e.expectProfileByUserID(profile)
			},
			shown: user,
		},
		{
			name:     "create by telegram id for existing user and profile",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WithArgs(int64(777001)).WillReturnRows(userRows(user))
				e.expectProfileByUserID(profile)
			},
			shown: user,
		},
		{
			name:     "create by telegram id for existing user without profile",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WithArgs(int64(777001)).WillReturnRows(userRows(user))
				e.mock.ExpectQuery(sqlProfileByUserID).WithArgs(user.ID).WillReturnError(sql.ErrNoRows)
				e.expectUserByID(user)
				e.mock.ExpectQuery(sqlInsertProfile).WithArgs(user.ID, "").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(profile.ID))
				e.expectProfileByUserID(profile)
			},
			shown: user,
		},
		{
			name:     "create by telegram id for unknown user",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WithArgs(int64(777001)).WillReturnError(sql.ErrNoRows)
				e.mock.ExpectQuery(sqlInsertUser).WithArgs(int64(777001), "", "", "").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newUser.ID))
				e.expectUserByID(newUser)
				e.expectProfileByUserID(repositories.Profile{ID: profile.ID, UserID: newUser.ID})
			},
			shown: newUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openMainMenu()
			e.mustClick(tt.callback)
			promptID := e.lastSentID()
			e.client.Reset()
			tt.setup(e)

			input := e.mustText(tt.input)

			assert.Equal(t, []int64{input.MessageId, promptID}, e.deletedMessageIDs())
			sends := e.client.CallsTo("sendMessage")
			require.Len(t, sends, 1)
			assertEditMenu(t, sends[0], tt.shown)

			// The found user and profile IDs are remembered for editing
			e.client.Reset()
			e.expectUserByID(tt.shown)
			e.expectProfileByID(repositories.Profile{ID: profile.ID, UserID: tt.shown.ID})
			e.mustClick(constants.AdminProfilesEditFirstnameCallback)
			assert.Contains(t, e.lastSendMessage().Params["text"], adminProfilesMenuEditFirstnameHeader)
		})
	}
}

func TestAdminProfilesHandler_EditMenuShowsProfileDetails(t *testing.T) {
	e := newProfilesEnv(t)
	user := sampleUser()
	user.HasCoffeeBan = true
	profile := sampleProfile()
	profile.Bio = "I <3 Go"
	profile.PublishedMessageID = sql.NullInt64{Int64: 4321, Valid: true}

	e.openProfile(user, profile)

	text := e.lastSentText()
	assert.Contains(t, text, "(@ivanp)")
	assert.Contains(t, text, "<blockquote expandable>I &lt;3 Go</blockquote>")
	assert.Contains(t, text, "<i>Карма:</i> <b>42</b>")
	assert.Contains(t, text, "<i>Кофейные встречи:</i> ❌ Запрещено")
	assert.Contains(t, text, "<i>Ссылка на профиль:</i> "+utils.GetIntroMessageLink(e.cfg, 4321))
}

func TestAdminProfilesHandler_InvalidOrUnknownSearchInput(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		input    string
		setup    func(e *testEnv)
		wantText string
	}{
		{
			name:     "unknown username",
			callback: constants.AdminProfilesSearchByUsernameCallback,
			input:    "@ghost",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByUsername).WithArgs("ghost").WillReturnError(sql.ErrNoRows)
			},
			wantText: "<b>" + adminProfilesMenuHeader + "</b>\n\nПользователь <b>ghost</b> не найден.\n\nПопробуй ещё раз, или вернись назад:",
		},
		{
			name:     "non numeric telegram id in search",
			callback: constants.AdminProfilesSearchByTelegramIDCallback,
			input:    "abc",
			setup:    func(e *testEnv) {},
			wantText: "<b>" + adminProfilesMenuSearchByIDHeader + "</b>\n\nНекорректный формат ID: <b>abc</b>. Введи числовой ID пользователя Telegram:",
		},
		{
			name:     "unknown telegram id in search",
			callback: constants.AdminProfilesSearchByTelegramIDCallback,
			input:    "555",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WithArgs(int64(555)).WillReturnError(sql.ErrNoRows)
			},
			wantText: "<b>" + adminProfilesMenuSearchByIDHeader + "</b>\n\nПользователь с ID <b>555</b> не найден.\n\nПопробуй ещё раз, или вернись назад:",
		},
		{
			name:     "single word name",
			callback: constants.AdminProfilesSearchByFullNameCallback,
			input:    "Ivan",
			setup:    func(e *testEnv) {},
			wantText: "<b>" + adminProfilesMenuSearchByNameHeader + "</b>\n\nНекорректный формат имени: <b>Ivan</b>.\n\nПожалуйста, введи имя и фамилию пользователя через пробел:",
		},
		{
			name:     "unknown full name",
			callback: constants.AdminProfilesSearchByFullNameCallback,
			input:    "Ivan Petrov Jr",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByName).WithArgs("Ivan", "Petrov Jr").WillReturnError(sql.ErrNoRows)
			},
			wantText: "<b>" + adminProfilesMenuSearchByNameHeader + "</b>\n\nПользователь с именем <b>Ivan Petrov Jr</b> не найден.\n\nПопробуй ещё раз, или вернись назад:",
		},
		{
			name:     "non numeric telegram id in create",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "12ab",
			setup:    func(e *testEnv) {},
			wantText: "<b>" + adminProfilesMenuCreateByIDHeader + "</b>\n\nНекорректный формат ID: <b>12ab</b>. Введи числовой ID пользователя Telegram:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openMainMenu()
			e.mustClick(tt.callback)
			prevID := e.lastSentID()

			// The same input twice: the admin stays in the same state and every retry replaces the previous answer
			for attempt := 0; attempt < 2; attempt++ {
				e.client.Reset()
				tt.setup(e)

				input := e.mustText(tt.input)

				assert.Equal(t, []int64{prevID, input.MessageId}, e.deletedMessageIDs(), "attempt %d", attempt)
				call := e.lastSendMessage()
				assert.Equal(t, tt.wantText, call.Params["text"])
				assert.Equal(t, "HTML", call.Params["parse_mode"])
				assert.Equal(t, buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback), markup(t, call))
				prevID = e.lastSentID()
			}
		})
	}
}

func TestAdminProfilesHandler_SearchInputDBErrors(t *testing.T) {
	tests := []struct {
		name      string
		callback  string
		input     string
		setup     func(e *testEnv)
		wantErr   string
		wantReply string
	}{
		{
			name:     "username lookup fails",
			callback: constants.AdminProfilesSearchByUsernameCallback,
			input:    "ivanp",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByUsername).WillReturnError(errDB)
			},
			wantErr: "failed to get user in handleUsernameInput",
		},
		{
			name:     "username profile lookup fails",
			callback: constants.AdminProfilesSearchByUsernameCallback,
			input:    "ivanp",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByUsername).WillReturnRows(userRows(sampleUser()))
				e.mock.ExpectQuery(sqlProfileByUserID).WillReturnError(errDB)
			},
			wantErr:   "failed to get/create profile in handleUsernameInput",
			wantReply: "Произошла ошибка при получении или создании профиля.",
		},
		{
			name:     "telegram id lookup fails",
			callback: constants.AdminProfilesSearchByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnError(errDB)
			},
			wantErr: "failed to get user in handleSearchUserIDInput",
		},
		{
			name:     "telegram id profile lookup fails",
			callback: constants.AdminProfilesSearchByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnRows(userRows(sampleUser()))
				e.mock.ExpectQuery(sqlProfileByUserID).WillReturnError(errDB)
			},
			wantErr:   "failed to get profile in handleSearchUserIDInput",
			wantReply: "Произошла ошибка при получении профиля.",
		},
		{
			name:     "full name lookup fails",
			callback: constants.AdminProfilesSearchByFullNameCallback,
			input:    "Ivan Petrov",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByName).WillReturnError(errDB)
			},
			wantErr: "failed to search user in handleSearchNameInput",
		},
		{
			name:     "full name profile lookup fails",
			callback: constants.AdminProfilesSearchByFullNameCallback,
			input:    "Ivan Petrov",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByName).WillReturnRows(userRows(sampleUser()))
				e.mock.ExpectQuery(sqlProfileByUserID).WillReturnError(errDB)
			},
			wantErr:   "failed to get profile in handleSearchNameInput",
			wantReply: "Произошла ошибка при получении профиля.",
		},
		{
			name:     "create by id lookup fails",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnError(errDB)
			},
			wantErr: "failed to get user in handleUserIDInput",
		},
		{
			name:     "create by id user insert fails",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnError(sql.ErrNoRows)
				e.mock.ExpectQuery(sqlInsertUser).WillReturnError(errDB)
			},
			wantErr: "failed to create user in handleUserIDInput",
		},
		{
			name:     "create by id created user lookup fails",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnError(sql.ErrNoRows)
				e.mock.ExpectQuery(sqlInsertUser).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
				e.mock.ExpectQuery(sqlUserByID).WithArgs(11).WillReturnError(errDB)
			},
			wantErr: "failed to get created user in handleUserIDInput",
		},
		{
			name:     "create by id profile creation fails",
			callback: constants.AdminProfilesCreateByTelegramIDCallback,
			input:    "777001",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnRows(userRows(sampleUser()))
				e.mock.ExpectQuery(sqlProfileByUserID).WillReturnError(errDB)
			},
			wantErr:   "failed to get/create profile in handleUserIDInput",
			wantReply: "Произошла ошибка при получении или создании профиля.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openMainMenu()
			e.mustClick(tt.callback)
			e.client.Reset()
			tt.setup(e)

			input, err := e.text(tt.input)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			if tt.wantReply == "" {
				assert.Empty(t, e.client.CallsTo("sendMessage"))
			} else {
				assert.Equal(t, []string{tt.wantReply}, e.client.SentTexts())
				var reply gotgbot.ReplyParameters
				require.NoError(t, e.lastSendMessage().DecodeJSON("reply_parameters", &reply))
				assert.Equal(t, input.MessageId, reply.MessageId)
			}
			assert.Empty(t, e.client.CallsTo("deleteMessage"))
		})
	}
}

func forwardedMessage(origin gotgbot.MessageOrigin, text string) *gotgbot.Message {
	msg := testutils.NewPrivateMessage(testAdminID, text)
	msg.ForwardOrigin = origin
	return msg
}

func TestAdminProfilesHandler_CreateByForwardedMessage_RejectsNonUserOrigin(t *testing.T) {
	const notForwardedText = "Это не пересланное сообщение от пользователя. Пожалуйста, перешли сообщение от пользователя, для которого нужно создать профиль:"
	const hiddenUserNote = "<i>Сообщение от скрытого пользователя не может быть использовано для создания профиля.</i>"

	tests := []struct {
		name       string
		origin     gotgbot.MessageOrigin
		wantHidden bool
	}{
		{
			name:       "hidden user",
			origin:     gotgbot.MessageOriginHiddenUser{Date: 1690000000, SenderUserName: "Anonymous"},
			wantHidden: true,
		},
		{
			name:   "channel post",
			origin: gotgbot.MessageOriginChannel{Date: 1690000000, Chat: gotgbot.Chat{Id: -100777, Type: "channel"}, MessageId: 5},
		},
		{
			name:   "anonymous group admin",
			origin: gotgbot.MessageOriginChat{Date: 1690000000, SenderChat: gotgbot.Chat{Id: -100888, Type: "supergroup"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openMainMenu()
			e.mustClick(constants.AdminProfilesCreateByForwardedMessageCallback)
			promptID := e.lastSentID()
			e.client.Reset()

			msg := forwardedMessage(tt.origin, "some forwarded text")
			require.NoError(t, e.dispatch(testutils.MessageUpdate(msg)))

			assert.Equal(t, []int64{promptID, msg.MessageId}, e.deletedMessageIDs())
			call := e.lastSendMessage()
			assert.True(t, strings.HasPrefix(call.Params["text"], "<b>"+adminProfilesMenuHeader+"</b>\n\n"+notForwardedText))
			if tt.wantHidden {
				assert.Contains(t, call.Params["text"], hiddenUserNote)
			} else {
				assert.NotContains(t, call.Params["text"], hiddenUserNote)
			}
			assert.Equal(t, buttons.ProfilesBackCancelButtons(constants.AdminProfilesStartCallback), markup(t, call))
		})
	}
}

func TestAdminProfilesHandler_CreateByForwardedMessage_CreatesUserAndProfile(t *testing.T) {
	e := newProfilesEnv(t)
	forwardedUser := gotgbot.User{Id: 777002, FirstName: "Anna", LastName: "Smirnova", Username: "annas"}
	dbUser := repositories.User{ID: 12, TgID: 777002, Firstname: "Anna", Lastname: "Smirnova", TgUsername: "annas"}
	const bio = "Hi, I'm Anna, a <Go> developer"
	dbProfile := repositories.Profile{ID: 22, UserID: 12, Bio: bio}

	e.openMainMenu()
	e.mustClick(constants.AdminProfilesCreateByForwardedMessageCallback)
	promptID := e.lastSentID()
	e.client.Reset()

	// Unknown user is created from the forwarded sender, the forwarded text becomes the bio
	e.mock.ExpectQuery(sqlUserByTgID).WithArgs(int64(777002)).WillReturnError(sql.ErrNoRows)
	e.mock.ExpectQuery(sqlInsertUser).WithArgs(int64(777002), "Anna", "Smirnova", "annas").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12))
	e.expectUserByID(dbUser)
	e.mock.ExpectQuery(sqlProfileByUserID).WithArgs(12).WillReturnError(sql.ErrNoRows)
	e.expectUserByID(dbUser)
	e.mock.ExpectQuery(sqlInsertProfile).WithArgs(12, bio).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(22))
	e.expectProfileByUserID(dbProfile)

	msg := forwardedMessage(gotgbot.MessageOriginUser{Date: 1690000000, SenderUser: forwardedUser}, bio)
	require.NoError(t, e.dispatch(testutils.MessageUpdate(msg)))

	assert.Equal(t, []int64{msg.MessageId, promptID}, e.deletedMessageIDs())
	call := e.lastSendMessage()
	assertEditMenu(t, call, dbUser)
	assert.Contains(t, call.Params["text"], "(@annas)")
	assert.Contains(t, call.Params["text"], "Hi, I'm Anna, a &lt;Go&gt; developer")

	// The new profile can be edited right away
	e.client.Reset()
	e.expectUserByID(dbUser)
	e.expectProfileByID(dbProfile)
	e.mustClick(constants.AdminProfilesEditBioCallback)
	assert.Contains(t, e.lastSendMessage().Params["text"], "<pre>Hi, I'm Anna, a &lt;Go&gt; developer</pre>")
}

func TestAdminProfilesHandler_CreateByForwardedMessage_DBErrors(t *testing.T) {
	forwardedUser := gotgbot.User{Id: 777001, FirstName: "Ivan", LastName: "Petrov", Username: "ivanp"}

	tests := []struct {
		name      string
		setup     func(e *testEnv)
		wantErr   string
		wantReply string
	}{
		{
			name: "user lookup fails",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnError(errDB)
			},
			wantErr:   "failed to create user in handleForwardedMessage",
			wantReply: "Произошла ошибка при создании пользователя.",
		},
		{
			name: "profile lookup fails",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnRows(userRows(sampleUser()))
				e.mock.ExpectQuery(sqlProfileByUserID).WillReturnError(errDB)
			},
			wantErr:   "failed to create profile in handleForwardedMessage",
			wantReply: "Произошла ошибка при создании профиля.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openMainMenu()
			e.mustClick(constants.AdminProfilesCreateByForwardedMessageCallback)
			e.client.Reset()
			tt.setup(e)

			msg := forwardedMessage(gotgbot.MessageOriginUser{Date: 1690000000, SenderUser: forwardedUser}, "bio")
			err := e.dispatch(testutils.MessageUpdate(msg))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Equal(t, []string{tt.wantReply}, e.client.SentTexts())
			assert.Empty(t, e.client.CallsTo("deleteMessage"))
		})
	}
}

func TestAdminProfilesHandler_EditFieldPrompts(t *testing.T) {
	bannedUser := sampleUser()
	bannedUser.HasCoffeeBan = true
	htmlProfile := sampleProfile()
	htmlProfile.Bio = "Loves <Go> & Rust"

	backCancel := buttons.ProfilesBackCancelButtons(constants.AdminProfilesEditMenuCallback)

	tests := []struct {
		name       string
		callback   string
		user       repositories.User
		profile    repositories.Profile
		wantText   string
		wantMarkup gotgbot.InlineKeyboardMarkup
	}{
		{
			name:       "first name",
			callback:   constants.AdminProfilesEditFirstnameCallback,
			user:       sampleUser(),
			profile:    sampleProfile(),
			wantText:   "<b>" + adminProfilesMenuEditFirstnameHeader + "</b>\n\nТекущее значение: <code>Ivan</code>\n\nВведи новое значение для поля <b>имя</b>",
			wantMarkup: backCancel,
		},
		{
			name:       "last name",
			callback:   constants.AdminProfilesEditLastnameCallback,
			user:       sampleUser(),
			profile:    sampleProfile(),
			wantText:   "<b>" + adminProfilesMenuEditLastnameHeader + "</b>\n\nТекущее значение: <code>Petrov</code>\n\nВведи новое значение для поля <b>фамилия</b>",
			wantMarkup: backCancel,
		},
		{
			name:       "username",
			callback:   constants.AdminProfilesEditUsernameCallback,
			user:       sampleUser(),
			profile:    sampleProfile(),
			wantText:   "<b>" + adminProfilesMenuEditUsernameHeader + "</b>\n\nТекущее значение: <code>ivanp</code>\n\nВведи новое значение для поля <b>username</b> (без @)",
			wantMarkup: backCancel,
		},
		{
			name:     "bio is html-escaped",
			callback: constants.AdminProfilesEditBioCallback,
			user:     sampleUser(),
			profile:  htmlProfile,
			wantText: "<b>" + adminProfilesMenuEditBioHeader + "</b>\n\nТекущее значение: <pre>Loves &lt;Go&gt; & Rust</pre>\n\n" +
				fmt.Sprintf("Введи новое значение для поля <b>биографию</b> (до %d символов)", constants.ProfileBioLengthLimit),
			wantMarkup: backCancel,
		},
		{
			name:       "coffee meetings allowed",
			callback:   constants.AdminProfilesEditCoffeeBanCallback,
			user:       sampleUser(),
			profile:    sampleProfile(),
			wantText:   "<b>" + adminProfilesMenuCoffeeBanHeader + "</b>\n\nТекущее значение: ✅ Разрешено\n\nНажмите на кнопку, чтобы изменить статус кофейных встреч",
			wantMarkup: buttons.ProfilesCoffeeBanButtons(constants.AdminProfilesEditMenuCallback, false),
		},
		{
			name:       "coffee meetings banned",
			callback:   constants.AdminProfilesEditCoffeeBanCallback,
			user:       bannedUser,
			profile:    sampleProfile(),
			wantText:   "<b>" + adminProfilesMenuCoffeeBanHeader + "</b>\n\nТекущее значение: ❌ Запрещено\n\nНажмите на кнопку, чтобы изменить статус кофейных встреч",
			wantMarkup: buttons.ProfilesCoffeeBanButtons(constants.AdminProfilesEditMenuCallback, true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			menuID := e.openProfile(tt.user, tt.profile)
			e.expectUserByID(tt.user)
			e.expectProfileByID(tt.profile)

			e.mustClick(tt.callback)

			assert.Equal(t, []int64{menuID}, e.deletedMessageIDs())
			call := e.lastSendMessage()
			assert.Equal(t, tt.wantText, call.Params["text"])
			assert.Equal(t, "HTML", call.Params["parse_mode"])
			assert.Equal(t, tt.wantMarkup, markup(t, call))
			promptID := e.lastSentID()

			// "Back" returns to the profile edit menu with fresh data
			e.client.Reset()
			e.expectUserByID(tt.user)
			e.expectProfileByUserID(tt.profile)
			e.mustClick(constants.AdminProfilesEditMenuCallback)

			assert.Equal(t, []int64{promptID}, e.deletedMessageIDs())
			assertEditMenu(t, e.lastSendMessage(), tt.user)
		})
	}
}

func TestAdminProfilesHandler_EditFieldDBErrors(t *testing.T) {
	user, profile := sampleUser(), sampleProfile()

	tests := []struct {
		name    string
		setup   func(e *testEnv)
		wantErr string
	}{
		{
			name: "user lookup fails",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByID).WillReturnError(errDB)
			},
			wantErr: "failed to get user in handleCallbackEditField",
		},
		{
			name: "profile lookup fails",
			setup: func(e *testEnv) {
				e.expectUserByID(user)
				e.mock.ExpectQuery(sqlProfileByID).WillReturnError(errDB)
			},
			wantErr: "failed to get profile in handleCallbackEditField",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openProfile(user, profile)
			tt.setup(e)

			err := e.click(constants.AdminProfilesEditBioCallback)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, e.client.CallsTo("sendMessage"))
			assert.Empty(t, e.client.CallsTo("deleteMessage"))
		})
	}
}

func TestAdminProfilesHandler_BackToEditMenuDBErrors(t *testing.T) {
	user, profile := sampleUser(), sampleProfile()

	tests := []struct {
		name    string
		setup   func(e *testEnv)
		wantErr string
	}{
		{
			name: "user lookup fails",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByID).WillReturnError(errDB)
			},
			wantErr: "failed to get user in handleEditMenuCallback",
		},
		{
			name: "profile lookup fails",
			setup: func(e *testEnv) {
				e.expectUserByID(user)
				e.mock.ExpectQuery(sqlProfileByUserID).WillReturnError(errDB)
			},
			wantErr: "failed to get profile in handleEditMenuCallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openField(user, profile, constants.AdminProfilesEditFirstnameCallback)
			tt.setup(e)

			err := e.click(constants.AdminProfilesEditMenuCallback)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, e.client.CallsTo("sendMessage"))
		})
	}
}

func TestAdminProfilesHandler_EditMenuStartReturnsToMainMenu(t *testing.T) {
	e := newProfilesEnv(t)
	menuID := e.openProfile(sampleUser(), sampleProfile())

	e.mustClick(constants.AdminProfilesStartCallback)

	assert.Equal(t, []int64{menuID}, e.deletedMessageIDs())
	assertMainMenu(t, e.lastSendMessage())
}

func TestAdminProfilesHandler_FieldInputSaved(t *testing.T) {
	// Every successful save waits 1 second before showing the profile again, so the cases run in parallel
	tests := []struct {
		name      string
		callback  string
		input     string
		expectSQL func(e *testEnv)
		updated   func(u *repositories.User, p *repositories.Profile)
		wantShown string
	}{
		{
			name:     "first name",
			callback: constants.AdminProfilesEditFirstnameCallback,
			input:    "Иван",
			expectSQL: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE users SET updated_at = NOW\(\), firstname = \$1 WHERE id = \$2`).
					WithArgs("Иван", 11).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			updated:   func(u *repositories.User, _ *repositories.Profile) { u.Firstname = "Иван" },
			wantShown: "Иван Petrov",
		},
		{
			name:     "last name",
			callback: constants.AdminProfilesEditLastnameCallback,
			input:    "Petrov-Vodkin",
			expectSQL: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE users SET updated_at = NOW\(\), lastname = \$1 WHERE id = \$2`).
					WithArgs("Petrov-Vodkin", 11).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			updated:   func(u *repositories.User, _ *repositories.Profile) { u.Lastname = "Petrov-Vodkin" },
			wantShown: "Ivan Petrov-Vodkin",
		},
		{
			name:     "username without @ prefix",
			callback: constants.AdminProfilesEditUsernameCallback,
			input:    "@" + strings.Repeat("u", 32),
			expectSQL: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE users SET updated_at = NOW\(\), tg_username = \$1 WHERE id = \$2`).
					WithArgs(strings.Repeat("u", 32), 11).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			updated:   func(u *repositories.User, _ *repositories.Profile) { u.TgUsername = strings.Repeat("u", 32) },
			wantShown: "(@" + strings.Repeat("u", 32) + ")",
		},
		{
			name:     "bio",
			callback: constants.AdminProfilesEditBioCallback,
			input:    "Backend engineer, loves Go",
			expectSQL: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE profiles SET updated_at = NOW\(\), bio = \$1 WHERE id = \$2`).
					WithArgs("Backend engineer, loves Go", 21).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			updated:   func(_ *repositories.User, p *repositories.Profile) { p.Bio = "Backend engineer, loves Go" },
			wantShown: "<blockquote expandable>Backend engineer, loves Go</blockquote>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newProfilesEnv(t)
			user, profile := sampleUser(), sampleProfile()
			promptID := e.openField(user, profile, tt.callback)

			updatedUser, updatedProfile := user, profile
			tt.updated(&updatedUser, &updatedProfile)
			tt.expectSQL(e)
			e.expectUserByID(updatedUser)
			e.expectProfileByID(updatedProfile)

			input := e.mustText(tt.input)

			successID := e.sentIDByText("✅ Значение успешно сохранено!")
			assert.Equal(t, []int64{input.MessageId, promptID, successID}, e.deletedMessageIDs())
			sends := e.client.CallsTo("sendMessage")
			require.Len(t, sends, 2)
			assert.Equal(t, "✅ Значение успешно сохранено!", sends[0].Params["text"])
			assertEditMenu(t, sends[1], updatedUser)
			assert.Contains(t, sends[1].Params["text"], tt.wantShown)

			// Back in the edit menu state: "Start" is only available there
			e.client.Reset()
			e.mustClick(constants.AdminProfilesStartCallback)
			assertMainMenu(t, e.lastSendMessage())
		})
	}
}

// privateMessageAt builds an admin message with a custom date (the bio handler deduplicates by date)
func privateMessageAt(text string, date int64) *gotgbot.Message {
	msg := testutils.NewPrivateMessage(testAdminID, text)
	msg.Date = date
	return msg
}

func TestAdminProfilesHandler_FieldInputTooLong(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		input    string
		wantText string
	}{
		{
			name:     "first name longer than 30",
			callback: constants.AdminProfilesEditFirstnameCallback,
			input:    strings.Repeat("a", 31),
			wantText: "<b>" + adminProfilesMenuEditFirstnameHeader + "</b>\n\nИмя слишком длинное. Пожалуйста, введи более короткое имя (не более 30 символов):",
		},
		{
			name:     "last name longer than 30",
			callback: constants.AdminProfilesEditLastnameCallback,
			input:    strings.Repeat("b", 31),
			wantText: "<b>" + adminProfilesMenuEditLastnameHeader + "</b>\n\nФамилия слишком длинная. Пожалуйста, введи более короткую фамилию (не более 30 символов):",
		},
		{
			name:     "username longer than 32",
			callback: constants.AdminProfilesEditUsernameCallback,
			input:    "@" + strings.Repeat("c", 33),
			wantText: "<b>" + adminProfilesMenuEditUsernameHeader + "</b>\n\nUsername слишком длинный. Пожалуйста, введи более короткий username (не более 32 символов):",
		},
		{
			name:     "bio longer than limit in UTF-16 code units",
			callback: constants.AdminProfilesEditBioCallback,
			// 1951 emojis = 3902 UTF-16 code units
			input: strings.Repeat("😀", 1951),
			wantText: "<b>" + adminProfilesMenuEditBioHeader + "</b>\n\nТекущая длина: 3902 символов" +
				fmt.Sprintf("\n\nПожалуйста, сократи до %d символов и пришли снова:", constants.ProfileBioLengthLimit),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			prevID := e.openField(sampleUser(), sampleProfile(), tt.callback)

			// Each retry replaces the previous error message and the admin stays in the same state
			for attempt := int64(0); attempt < 2; attempt++ {
				e.client.Reset()
				input := privateMessageAt(tt.input, 1700000000+attempt)
				require.NoError(t, e.dispatch(testutils.MessageUpdate(input)))

				assert.Equal(t, []int64{prevID, input.MessageId}, e.deletedMessageIDs(), "attempt %d", attempt)
				call := e.lastSendMessage()
				assert.Equal(t, tt.wantText, call.Params["text"])
				assert.Equal(t, buttons.ProfilesBackCancelButtons(constants.AdminProfilesEditMenuCallback), markup(t, call))
				prevID = e.lastSentID()
			}
		})
	}
}

func TestAdminProfilesHandler_BioInput_SkipsMessagePartsWithSameDate(t *testing.T) {
	e := newProfilesEnv(t)
	e.openField(sampleUser(), sampleProfile(), constants.AdminProfilesEditBioCallback)

	// A long text is split by Telegram into several messages with the same date
	first := privateMessageAt(strings.Repeat("x", constants.ProfileBioLengthLimit+1), 1700000100)
	require.NoError(t, e.dispatch(testutils.MessageUpdate(first)))
	require.Len(t, e.client.CallsTo("sendMessage"), 1)
	e.client.Reset()

	second := privateMessageAt("tail of the long text", 1700000100)
	require.NoError(t, e.dispatch(testutils.MessageUpdate(second)))

	assert.Equal(t, []int64{second.MessageId}, e.deletedMessageIDs())
	assert.Empty(t, e.client.CallsTo("sendMessage"))
}

func TestAdminProfilesHandler_FieldInputDBErrors(t *testing.T) {
	user, profile := sampleUser(), sampleProfile()

	tests := []struct {
		name     string
		callback string
		input    string
		setup    func(e *testEnv)
		wantErr  string
	}{
		{
			name:     "first name update fails",
			callback: constants.AdminProfilesEditFirstnameCallback,
			input:    "Ivan",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE users SET`).WillReturnError(errDB)
			},
			wantErr: "failed to update firstname",
		},
		{
			name:     "last name update matches no rows",
			callback: constants.AdminProfilesEditLastnameCallback,
			input:    "Petrov",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE users SET`).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: "failed to update lastname",
		},
		{
			name:     "username update fails",
			callback: constants.AdminProfilesEditUsernameCallback,
			input:    "ivanp",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE users SET`).WillReturnError(errDB)
			},
			wantErr: "failed to update username",
		},
		{
			name:     "bio update fails",
			callback: constants.AdminProfilesEditBioCallback,
			input:    "bio",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE profiles SET`).WillReturnError(errDB)
			},
			wantErr: "failed to update bio",
		},
		{
			name:     "reloading user fails",
			callback: constants.AdminProfilesEditFirstnameCallback,
			input:    "Ivan",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE users SET`).WillReturnResult(sqlmock.NewResult(0, 1))
				e.mock.ExpectQuery(sqlUserByID).WillReturnError(errDB)
			},
			wantErr: "failed to get user in returnToProfileView",
		},
		{
			name:     "reloading profile fails",
			callback: constants.AdminProfilesEditBioCallback,
			input:    "bio",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(`UPDATE profiles SET`).WillReturnResult(sqlmock.NewResult(0, 1))
				e.expectUserByID(user)
				e.mock.ExpectQuery(sqlProfileByID).WillReturnError(errDB)
			},
			wantErr: "failed to get profile in returnToProfileView",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openField(user, profile, tt.callback)
			tt.setup(e)

			_, err := e.text(tt.input)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			for _, text := range e.client.SentTexts() {
				assert.NotContains(t, text, "успешно сохранено")
			}
		})
	}
}

func TestAdminProfilesHandler_ToggleCoffeeBan(t *testing.T) {
	e := newProfilesEnv(t)
	user, profile := sampleUser(), sampleProfile()
	promptID := e.openField(user, profile, constants.AdminProfilesEditCoffeeBanCallback)

	// Allowed -> banned
	e.expectUserByID(user)
	e.mock.ExpectExec(sqlSetCoffeeBan).WithArgs(true, 11).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mustClick(constants.AdminProfilesToggleCoffeeBanCallback)

	assert.Equal(t, []int64{promptID}, e.deletedMessageIDs())
	call := e.lastSendMessage()
	assert.True(t, strings.HasPrefix(call.Params["text"], "<b>"+adminProfilesMenuCoffeeBanHeader+"</b>"))
	assert.Equal(t, buttons.ProfilesCoffeeBanButtons(constants.AdminProfilesEditMenuCallback, true), markup(t, call))
	promptID = e.lastSentID()

	// Banned -> allowed
	e.client.Reset()
	banned := user
	banned.HasCoffeeBan = true
	e.expectUserByID(banned)
	e.mock.ExpectExec(sqlSetCoffeeBan).WithArgs(false, 11).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mustClick(constants.AdminProfilesToggleCoffeeBanCallback)

	assert.Equal(t, []int64{promptID}, e.deletedMessageIDs())
	assert.Equal(t, buttons.ProfilesCoffeeBanButtons(constants.AdminProfilesEditMenuCallback, false), markup(t, e.lastSendMessage()))

	// Still in the coffee ban menu: "Back" returns to the profile
	e.client.Reset()
	e.expectUserByID(user)
	e.expectProfileByUserID(profile)
	e.mustClick(constants.AdminProfilesEditMenuCallback)
	assertEditMenu(t, e.lastSendMessage(), user)
}

func TestAdminProfilesHandler_ToggleCoffeeBanDBErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(e *testEnv)
		wantErr string
	}{
		{
			name: "user lookup fails",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByID).WillReturnError(errDB)
			},
			wantErr: "failed to get user in handleToggleCoffeeBanCallback",
		},
		{
			name: "update fails",
			setup: func(e *testEnv) {
				e.expectUserByID(sampleUser())
				e.mock.ExpectExec(sqlSetCoffeeBan).WillReturnError(errDB)
			},
			wantErr: "failed to update coffee ban status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openField(sampleUser(), sampleProfile(), constants.AdminProfilesEditCoffeeBanCallback)
			tt.setup(e)

			err := e.click(constants.AdminProfilesToggleCoffeeBanCallback)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, e.client.CallsTo("sendMessage"))
			assert.Empty(t, e.client.CallsTo("deleteMessage"))
		})
	}
}

func TestAdminProfilesHandler_PublishIncompleteProfile(t *testing.T) {
	tests := []struct {
		name    string
		user    func(u *repositories.User)
		profile func(p *repositories.Profile)
		want    []string
	}{
		{
			name:    "missing last name and bio",
			user:    func(u *repositories.User) { u.Lastname = "" },
			profile: func(p *repositories.Profile) { p.Bio = "" },
			want:    []string{"└ ✅ Имя", "└ ❌ Фамилию", "└ ❌ Биографию"},
		},
		{
			name:    "missing first name",
			user:    func(u *repositories.User) { u.Firstname = "" },
			profile: func(p *repositories.Profile) {},
			want:    []string{"└ ❌ Имя", "└ ✅ Фамилию", "└ ✅ Биографию"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			user, profile := sampleUser(), sampleProfile()
			tt.user(&user)
			tt.profile(&profile)
			menuID := e.openProfile(user, profile)
			e.expectUserByID(user)
			e.expectProfileByID(profile)

			e.mustClick(constants.AdminProfilesPublishCallback)

			sends := e.client.CallsTo("sendMessage")
			require.Len(t, sends, 1, "nothing must be published to the group")
			assert.Equal(t, testAdminID, sends[0].Int64("chat_id"))
			text := sends[0].Params["text"]
			assert.True(t, strings.HasPrefix(text, "<b>"+adminProfilesMenuPublishHeader+"</b>\n\n⚠️ Профиль пользователя неполный."))
			assert.Contains(t, text, fmt.Sprintf("<a href='%s'>Интро</a>", utils.GetIntroTopicLink(e.cfg)))
			for _, line := range tt.want {
				assert.Contains(t, text, "\n"+line)
			}
			assert.Equal(t, buttons.ProfilesBackCancelButtons(constants.AdminProfilesEditMenuCallback), markup(t, sends[0]))
			assert.Equal(t, []int64{menuID}, e.deletedMessageIDs())
			assert.Empty(t, e.client.CallsTo("editMessageText"))
		})
	}
}

func TestAdminProfilesHandler_PublishNewMessage(t *testing.T) {
	tests := []struct {
		name            string
		callback        string
		previewDisabled bool
	}{
		{name: "with preview", callback: constants.AdminProfilesPublishCallback, previewDisabled: false},
		{name: "without preview", callback: constants.AdminProfilesPublishNoPreviewCallback, previewDisabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			user, profile := sampleUser(), sampleProfile()
			menuID := e.openProfile(user, profile)
			publishedID := e.nextSentID()
			e.expectUserByID(user)
			e.expectProfileByID(profile)
			e.mock.ExpectExec(sqlUpdatePublishedMsg).WithArgs(publishedID, profile.ID).WillReturnResult(sqlmock.NewResult(0, 1))

			e.mustClick(tt.callback)

			sends := e.client.CallsTo("sendMessage")
			require.Len(t, sends, 2)

			published := sends[0]
			assert.Equal(t, fullGroupChatID(), published.Int64("chat_id"))
			assert.Equal(t, int64(testIntroTopicID), published.Int64("message_thread_id"))
			assert.Equal(t, "HTML", published.Params["parse_mode"])
			assert.Equal(t, tt.previewDisabled, linkPreviewDisabled(t, published))
			assert.Contains(t, published.Params["text"], "🖐 <b><a href=\"tg://user?id=777001\">Ivan Petrov</a></b> (@ivanp)")
			assert.Contains(t, published.Params["text"], "<blockquote>О себе</blockquote>\nGo developer")

			success := sends[1]
			assert.Equal(t, testAdminID, success.Int64("chat_id"))
			assert.True(t, strings.HasPrefix(success.Params["text"],
				"<b>"+adminProfilesMenuPublishHeader+"</b>\n\n✅ Профиль пользователя успешно опубликован в канале"))
			assert.Equal(t, buttons.ProfilesBackStartCancelButtons(constants.AdminProfilesEditMenuCallback), markup(t, success))
			assert.Equal(t, []int64{menuID}, e.deletedMessageIDs())
			assert.Empty(t, e.client.CallsTo("editMessageText"))

			// "Back" from the success message returns to the profile
			e.client.Reset()
			e.expectUserByID(user)
			e.expectProfileByUserID(profile)
			e.mustClick(constants.AdminProfilesEditMenuCallback)
			assertEditMenu(t, e.lastSendMessage(), user)
		})
	}
}

func TestAdminProfilesHandler_PublishExistingMessage(t *testing.T) {
	const oldMessageID int64 = 3333

	tests := []struct {
		name          string
		editError     string
		wantRepublish bool
	}{
		{name: "edit succeeds"},
		{
			name:      "content did not change",
			editError: "Bad Request: message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message",
		},
		{
			name:          "old message is gone",
			editError:     "Bad Request: message to edit not found",
			wantRepublish: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			user, profile := sampleUser(), sampleProfile()
			profile.PublishedMessageID = sql.NullInt64{Int64: oldMessageID, Valid: true}
			e.openProfile(user, profile)
			if tt.editError != "" {
				e.client.OnError("editMessageText", tt.editError)
			}

			wantID := oldMessageID
			if tt.wantRepublish {
				wantID = e.nextSentID()
			}
			e.expectUserByID(user)
			e.expectProfileByID(profile)
			e.mock.ExpectExec(sqlUpdatePublishedMsg).WithArgs(wantID, profile.ID).WillReturnResult(sqlmock.NewResult(0, 1))

			e.mustClick(constants.AdminProfilesPublishNoPreviewCallback)

			edit, ok := e.client.LastCallTo("editMessageText")
			require.True(t, ok)
			assert.Equal(t, fullGroupChatID(), edit.Int64("chat_id"))
			assert.Equal(t, oldMessageID, edit.Int64("message_id"))
			assert.Equal(t, "HTML", edit.Params["parse_mode"])
			assert.True(t, linkPreviewDisabled(t, edit))
			assert.Contains(t, edit.Params["text"], "Ivan Petrov")

			var groupSends []testutils.APICall
			for _, c := range e.client.CallsTo("sendMessage") {
				if c.Int64("chat_id") == fullGroupChatID() {
					groupSends = append(groupSends, c)
				}
			}
			if tt.wantRepublish {
				require.Len(t, groupSends, 1)
				assert.Equal(t, int64(testIntroTopicID), groupSends[0].Int64("message_thread_id"))
				assert.Equal(t, edit.Params["text"], groupSends[0].Params["text"])
			} else {
				assert.Empty(t, groupSends)
				assert.Contains(t, e.lastSendMessage().Params["text"], utils.GetIntroMessageLink(e.cfg, oldMessageID))
			}
			assert.Contains(t, e.lastSendMessage().Params["text"], "успешно опубликован")
		})
	}
}

func TestAdminProfilesHandler_PublishErrors(t *testing.T) {
	user := sampleUser()
	published := sampleProfile()
	published.PublishedMessageID = sql.NullInt64{Int64: 3333, Valid: true}

	failGroupSends := func(chatID int64) string {
		if chatID < 0 {
			return "Bad Request: not enough rights to send text messages to the chat"
		}
		return ""
	}

	tests := []struct {
		name    string
		profile repositories.Profile
		setup   func(e *testEnv)
		wantErr string
		// whether the handler got as far as announcing the publication to the admin
		successAttempted bool
	}{
		{
			name:    "user lookup fails",
			profile: sampleProfile(),
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlUserByID).WillReturnError(errDB)
			},
			wantErr: "failed to get user in handlePublishProfile",
		},
		{
			name:    "profile lookup fails",
			profile: sampleProfile(),
			setup: func(e *testEnv) {
				e.expectUserByID(user)
				e.mock.ExpectQuery(sqlProfileByID).WillReturnError(errDB)
			},
			wantErr: "failed to get profile in handlePublishProfile",
		},
		{
			name:    "sending new message fails",
			profile: sampleProfile(),
			setup: func(e *testEnv) {
				e.expectUserByID(user)
				e.expectProfileByID(sampleProfile())
				e.setFailSend(failGroupSends)
			},
			wantErr: "failed to publish profile",
		},
		{
			name:    "re-sending after failed edit fails",
			profile: published,
			setup: func(e *testEnv) {
				e.expectUserByID(user)
				e.expectProfileByID(published)
				e.client.OnError("editMessageText", "Bad Request: message to edit not found")
				e.setFailSend(failGroupSends)
			},
			wantErr: "failed to publish profile",
		},
		{
			name:    "saving published message id fails",
			profile: sampleProfile(),
			setup: func(e *testEnv) {
				e.expectUserByID(user)
				e.expectProfileByID(sampleProfile())
				e.mock.ExpectExec(sqlUpdatePublishedMsg).WillReturnError(errDB)
			},
			wantErr: "failed to update published message ID",
		},
		{
			name:    "success message fails",
			profile: sampleProfile(),
			setup: func(e *testEnv) {
				e.expectUserByID(user)
				e.expectProfileByID(sampleProfile())
				e.mock.ExpectExec(sqlUpdatePublishedMsg).WillReturnResult(sqlmock.NewResult(0, 1))
				e.setFailSend(func(chatID int64) string {
					if chatID == testAdminID {
						return "Forbidden: bot was blocked by the user"
					}
					return ""
				})
			},
			wantErr:          "failed to send success message",
			successAttempted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			e.openProfile(user, tt.profile)
			tt.setup(e)

			err := e.click(constants.AdminProfilesPublishCallback)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			attempted := false
			for _, text := range e.client.SentTexts() {
				if strings.Contains(text, "успешно опубликован") {
					attempted = true
				}
			}
			assert.Equal(t, tt.successAttempted, attempted)
		})
	}
}

func failAdminSends(chatID int64) string {
	if chatID == testAdminID {
		return "Forbidden: bot was blocked by the user"
	}
	return ""
}

func TestAdminProfilesHandler_SendFailuresAreReported(t *testing.T) {
	user, profile := sampleUser(), sampleProfile()
	incomplete := sampleProfile()
	incomplete.Bio = ""

	openSubMenu := func(callback string) func(e *testEnv) {
		return func(e *testEnv) {
			e.openMainMenu()
			e.mustClick(callback)
		}
	}
	textAct := func(text string) func(e *testEnv) error {
		return func(e *testEnv) error {
			_, err := e.text(text)
			return err
		}
	}
	clickAct := func(data string) func(e *testEnv) error {
		return func(e *testEnv) error { return e.click(data) }
	}

	tests := []struct {
		name    string
		prepare func(e *testEnv)
		act     func(e *testEnv) error
		wantErr string
	}{
		{
			name:    "main menu",
			prepare: func(e *testEnv) {},
			act:     textAct(profilesCommand()),
			wantErr: "failed to send message in showMainMenu",
		},
		{
			name:    "search by username prompt",
			prepare: func(e *testEnv) { e.openMainMenu() },
			act:     clickAct(constants.AdminProfilesSearchByUsernameCallback),
			wantErr: "failed to send message in handleCallbackEdit",
		},
		{
			name:    "search by telegram id prompt",
			prepare: func(e *testEnv) { e.openMainMenu() },
			act:     clickAct(constants.AdminProfilesSearchByTelegramIDCallback),
			wantErr: "failed to send message in handleSearchByIDCallback",
		},
		{
			name:    "search by full name prompt",
			prepare: func(e *testEnv) { e.openMainMenu() },
			act:     clickAct(constants.AdminProfilesSearchByFullNameCallback),
			wantErr: "failed to send message in handleSearchByNameCallback",
		},
		{
			name:    "create by forwarded message prompt",
			prepare: func(e *testEnv) { e.openMainMenu() },
			act:     clickAct(constants.AdminProfilesCreateByForwardedMessageCallback),
			wantErr: "failed to send message in handleCallbackCreate",
		},
		{
			name:    "create by telegram id prompt",
			prepare: func(e *testEnv) { e.openMainMenu() },
			act:     clickAct(constants.AdminProfilesCreateByTelegramIDCallback),
			wantErr: "failed to send message in handleCreateByIDCallback",
		},
		{
			name:    "create by telegram id with invalid id",
			prepare: openSubMenu(constants.AdminProfilesCreateByTelegramIDCallback),
			act:     textAct("x"),
			wantErr: "failed to send message in handleUserIDInput",
		},
		{
			name:    "search by telegram id with invalid id",
			prepare: openSubMenu(constants.AdminProfilesSearchByTelegramIDCallback),
			act:     textAct("x"),
			wantErr: "failed to send message in handleSearchUserIDInput",
		},
		{
			name: "search by telegram id with unknown id",
			prepare: func(e *testEnv) {
				openSubMenu(constants.AdminProfilesSearchByTelegramIDCallback)(e)
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnError(sql.ErrNoRows)
			},
			act:     textAct("1"),
			wantErr: "failed to send message in handleSearchUserIDInput",
		},
		{
			name: "search by unknown username",
			prepare: func(e *testEnv) {
				openSubMenu(constants.AdminProfilesSearchByUsernameCallback)(e)
				e.mock.ExpectQuery(sqlUserByUsername).WillReturnError(sql.ErrNoRows)
			},
			act:     textAct("ghost"),
			wantErr: "failed to send message in handleUsernameInput",
		},
		{
			name:    "search by malformed full name",
			prepare: openSubMenu(constants.AdminProfilesSearchByFullNameCallback),
			act:     textAct("Ivan"),
			wantErr: "failed to send message in handleSearchByFullNameInput",
		},
		{
			name: "search by unknown full name",
			prepare: func(e *testEnv) {
				openSubMenu(constants.AdminProfilesSearchByFullNameCallback)(e)
				e.mock.ExpectQuery(sqlUserByName).WillReturnError(sql.ErrNoRows)
			},
			act:     textAct("Ivan Petrov"),
			wantErr: "failed to send message in handleSearchNameInput",
		},
		{
			name:    "forward from hidden user",
			prepare: openSubMenu(constants.AdminProfilesCreateByForwardedMessageCallback),
			act: func(e *testEnv) error {
				msg := forwardedMessage(gotgbot.MessageOriginHiddenUser{Date: 1, SenderUserName: "Anon"}, "hi")
				return e.dispatch(testutils.MessageUpdate(msg))
			},
			wantErr: "failed to send message in handleForwardedMessage",
		},
		{
			name: "profile edit menu",
			prepare: func(e *testEnv) {
				openSubMenu(constants.AdminProfilesSearchByTelegramIDCallback)(e)
				e.mock.ExpectQuery(sqlUserByTgID).WillReturnRows(userRows(user))
				e.expectProfileByUserID(profile)
			},
			act:     textAct("777001"),
			wantErr: "failed to send message in showProfileEditMenu",
		},
		{
			name: "edit field prompt",
			prepare: func(e *testEnv) {
				e.openProfile(user, profile)
				e.expectUserByID(user)
				e.expectProfileByID(profile)
			},
			act:     clickAct(constants.AdminProfilesEditLastnameCallback),
			wantErr: "failed to send message in handleCallbackEditField",
		},
		{
			name: "coffee ban toggle",
			prepare: func(e *testEnv) {
				e.openField(user, profile, constants.AdminProfilesEditCoffeeBanCallback)
				e.expectUserByID(user)
				e.mock.ExpectExec(sqlSetCoffeeBan).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			act:     clickAct(constants.AdminProfilesToggleCoffeeBanCallback),
			wantErr: "failed to send message in handleToggleCoffeeBanCallback",
		},
		{
			name: "incomplete profile warning",
			prepare: func(e *testEnv) {
				e.openProfile(user, incomplete)
				e.expectUserByID(user)
				e.expectProfileByID(incomplete)
			},
			act:     clickAct(constants.AdminProfilesPublishCallback),
			wantErr: "failed to send message in handlePublishProfile",
		},
		{
			name: "saved value confirmation",
			prepare: func(e *testEnv) {
				e.openField(user, profile, constants.AdminProfilesEditFirstnameCallback)
				e.mock.ExpectExec(`UPDATE users SET`).WillReturnResult(sqlmock.NewResult(0, 1))
				e.expectUserByID(user)
				e.expectProfileByID(profile)
			},
			act:     textAct("Ivan"),
			wantErr: "failed to send success message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newProfilesEnv(t)
			tt.prepare(e)
			e.client.Reset()
			e.setFailSend(failAdminSends)

			err := tt.act(e)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, err.Error(), "bot was blocked by the user")
		})
	}
}

func TestAdminProfilesHandler_TooLongInputWhenTelegramFails(t *testing.T) {
	e := newProfilesEnv(t)
	e.openField(sampleUser(), sampleProfile(), constants.AdminProfilesEditFirstnameCallback)
	e.setFailSend(failAdminSends)

	_, err := e.text(strings.Repeat("a", 31))
	require.NoError(t, err, "a failed error message must not break the conversation")

	// Telegram is back: the admin is still asked for the first name
	e.setFailSend(nil)
	e.client.Reset()
	_, err = e.text(strings.Repeat("a", 31))
	require.NoError(t, err)
	assert.Contains(t, e.lastSendMessage().Params["text"], "Имя слишком длинное")
}
