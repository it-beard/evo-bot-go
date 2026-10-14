package privatehandlers

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	updateUserFieldSQL      = `UPDATE users SET updated_at = NOW\(\), %s = \$1 WHERE id = \$2`
	updateProfileBioSQL     = `UPDATE profiles SET updated_at = NOW\(\), bio = \$1 WHERE id = \$2`
	updatePublishedIDSQL    = `UPDATE profiles SET published_message_id = \$1, updated_at = NOW\(\) WHERE id = \$2`
	profileSessionEndedText = "Сессия работы с профилями завершена."
)

var (
	// me is the DB user of testUserID; its username matches testutils.NewUser so GetOrCreate does not update it
	me        = testDBUser{ID: 7, TgID: testUserID, Firstname: "Иван", Lastname: "Петров", Username: "user501"}
	myProfile = testDBProfile{ID: 3, UserID: 7, Bio: "Люблю Go"}
)

func (d *deps) expectUser(u testDBUser) {
	d.sql.ExpectQuery(selectUserByTgID).WithArgs(u.TgID).WillReturnRows(userRows(u))
}

func (d *deps) expectProfile(p testDBProfile) {
	d.sql.ExpectQuery(selectProfileByUser).WithArgs(p.UserID).WillReturnRows(profileRows(p))
}

func (d *deps) expectUserWithProfile(u testDBUser, p testDBProfile) {
	d.expectUser(u)
	d.expectProfile(p)
}

// captureArg is a sqlmock argument matcher that remembers the actual value
type captureArg struct {
	mu    sync.Mutex
	value driver.Value
}

func (c *captureArg) Match(v driver.Value) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = v
	return true
}

func (c *captureArg) Value() driver.Value {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// openProfileMenu runs /profile for an existing user and returns the id of the menu message
func openProfileMenu(t *testing.T, d *deps, h ext.Handler, u testDBUser, p testDBProfile) int64 {
	t.Helper()
	d.expectUserWithProfile(u, p)
	require.True(t, d.dispatch(t, h, privateText(testUserID, "/profile")))
	return d.lastSentID(t, profileMenuHeader)
}

// openFieldEditor opens the menu and presses the button of a field, returning the id of the field prompt
func openFieldEditor(t *testing.T, d *deps, h ext.Handler, callback string, u testDBUser, p testDBProfile) int64 {
	t.Helper()
	openProfileMenu(t, d, h, u, p)
	d.expectUserWithProfile(u, p)
	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, callback)))
	return d.lastSentID(t, "Введи ")
}

// dispatchFieldValue sends a new field value. After a successful save the handler returns
// NextConversationState(profileStateEditMyProfile), a state that is not registered in the conversation,
// so gotgbot reports an "unknown state" error (reported as a production bug). The error is therefore
// not asserted here, only the side effects are.
func dispatchFieldValue(t *testing.T, d *deps, h ext.Handler, u *gotgbot.Update) {
	t.Helper()
	handled, _ := testutils.Dispatch(d.bot, h, u)
	require.True(t, handled)
}

func dbUser(u testDBUser) *repositories.User {
	return &repositories.User{ID: u.ID, TgID: u.TgID, Firstname: u.Firstname, Lastname: u.Lastname, TgUsername: u.Username}
}

func dbProfile(p testDBProfile) *repositories.Profile {
	profile := &repositories.Profile{ID: p.ID, UserID: p.UserID, Bio: p.Bio}
	if id, ok := p.PublishedID.(int64); ok {
		profile.PublishedMessageID = sql.NullInt64{Int64: id, Valid: true}
	}
	return profile
}

// ---------- entry point ----------

func TestProfileHandler_NonPrivateChatIsRejected(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()

	require.True(t, d.dispatch(t, h, groupText(testUserID, "/profile")))

	assertPrivateOnlyRejection(t, d.client)
	assert.False(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)))
}

func TestProfileHandler_NonClubMemberIsRejected(t *testing.T) {
	d := newDeps(t)
	d.client.SetMemberStatus(testUserID, "kicked")
	h := d.newProfileHandler()

	require.True(t, d.dispatch(t, h, privateText(testUserID, "/profile")))

	assertClubOnlyRejection(t, d.client)
	assert.False(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)))
}

func TestProfileHandler_MenuShowsFieldStatuses(t *testing.T) {
	tests := []struct {
		name         string
		emptyTgNames bool
		expect       func(d *deps)
		contains     []string
		notContains  []string
	}{
		{
			name: "complete published profile",
			expect: func(d *deps) {
				d.expectUserWithProfile(me, testDBProfile{ID: 3, UserID: 7, Bio: "Люблю Go", PublishedID: int64(555)})
			},
			contains: []string{
				"└ ✅ Имя <i>(Иван)</i>",
				"└ ✅ Фамилия <i>(Петров)</i>",
				"└ ✅ Биография",
				"👉 <a href='https://telegram.me/c/1234567890/13/555'>Ссылка</a> на твой профиль.",
			},
		},
		{
			name:         "empty profile",
			emptyTgNames: true,
			expect: func(d *deps) {
				d.expectUserWithProfile(testDBUser{ID: 7, TgID: testUserID, Username: "user501"}, testDBProfile{ID: 3, UserID: 7})
			},
			contains:    []string{"└ ❌ Имя", "└ ❌ Фамилия", "└ ❌ Биография"},
			notContains: []string{"✅", "Ссылка</a> на твой профиль"},
		},
		{
			name: "user cannot be loaded",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByTgID).WithArgs(testUserID).WillReturnError(errors.New("db down"))
			},
			contains:    []string{"└ ❌ Имя", "└ ❌ Фамилия", "└ ❌ Биография"},
			notContains: []string{"✅"},
		},
		{
			name: "profile cannot be loaded",
			expect: func(d *deps) {
				d.expectUser(me)
				d.sql.ExpectQuery(selectProfileByUser).WithArgs(me.ID).WillReturnError(errors.New("db down"))
			},
			contains:    []string{"└ ✅ Имя <i>(Иван)</i>", "└ ✅ Фамилия <i>(Петров)</i>", "└ ❌ Биография"},
			notContains: []string{"Ссылка</a> на твой профиль"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			tt.expect(d)
			msg := testutils.NewPrivateMessage(testUserID, "/profile")
			if tt.emptyTgNames {
				msg.From.FirstName, msg.From.LastName = "", ""
			}

			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(msg)))

			menu := lastSend(t, d.client)
			text := menu.Params["text"]
			assert.Equal(t, testUserID, menu.Int64("chat_id"))
			assert.Equal(t, "HTML", menu.Params["parse_mode"])
			assert.True(t, strings.HasPrefix(text, "<b>Меню \"Профиль\"</b>"))
			assert.Contains(t, text, "<a href='https://telegram.me/c/1234567890/13'>Интро</a>")
			for _, s := range tt.contains {
				assert.Contains(t, text, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, text, s)
			}
			assert.Equal(t, []string{
				constants.ProfileEditMyProfileCallback,
				constants.ProfileSearchProfileCallback,
				constants.ProfileFullCancel,
			}, callbackData(t, menu))
		})
	}
}

func TestProfileHandler_NewUserAndProfileAreCreated(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	created := testDBUser{ID: 7, TgID: testUserID, Firstname: "First", Lastname: "Last", Username: "user501"}

	d.sql.ExpectQuery(selectUserByTgID).WithArgs(testUserID).WillReturnRows(sqlmock.NewRows(userColumns))
	d.sql.ExpectQuery(`INSERT INTO users \(tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member\)`).
		WithArgs(testUserID, "First", "Last", "user501").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	d.sql.ExpectQuery(`FROM users\s+WHERE id = \$1`).WithArgs(7).WillReturnRows(userRows(created))
	d.sql.ExpectQuery(selectProfileByUser).WithArgs(7).WillReturnRows(sqlmock.NewRows(profileColumns))
	d.sql.ExpectQuery(`FROM users\s+WHERE id = \$1`).WithArgs(7).WillReturnRows(userRows(created))
	d.sql.ExpectQuery(`INSERT INTO profiles \(user_id, bio\)`).WithArgs(7, "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	d.expectProfile(testDBProfile{ID: 3, UserID: 7})

	require.True(t, d.dispatch(t, h, privateText(testUserID, "/profile")))

	text := lastSend(t, d.client).Params["text"]
	assert.Contains(t, text, "└ ✅ Имя <i>(First)</i>")
	assert.Contains(t, text, "└ ✅ Фамилия <i>(Last)</i>")
	assert.Contains(t, text, "└ ❌ Биография")
}

func TestProfileHandler_MenuSendFailure(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	d.expectUserWithProfile(me, myProfile)
	d.client.OnError("sendMessage", "Forbidden: bot was blocked by the user")

	handled, err := testutils.Dispatch(d.bot, h, privateText(testUserID, "/profile"))

	require.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send message in showProfileMenu")
	assert.False(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)),
		"the conversation is not started")
}

// ---------- navigation ----------

func TestProfileHandler_EditMenuAndBack(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	menuID := openProfileMenu(t, d, h, me, myProfile)
	d.client.Reset()

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)))

	assert.Equal(t, []int64{menuID}, deletedMessageIDs(d.client))
	editMenu := lastSend(t, d.client)
	assert.Equal(t, "<b>Профиль → Редактирование</b>\n\nВыбери, что бы ты хотел/а изменить:", editMenu.Params["text"])
	assert.Equal(t, "HTML", editMenu.Params["parse_mode"])
	assert.Equal(t, []string{
		constants.ProfileEditFirstnameCallback,
		constants.ProfileEditLastnameCallback,
		constants.ProfileEditBioCallback,
		constants.ProfileStartCallback,
		constants.ProfileFullCancel,
	}, callbackData(t, editMenu))
	editMenuID := d.lastSentID(t, profileMenuEditHeader)

	// "Back" shows the main menu again
	d.client.Reset()
	d.expectUserWithProfile(me, myProfile)
	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileStartCallback)))

	assert.Equal(t, []int64{editMenuID}, deletedMessageIDs(d.client))
	assert.True(t, strings.HasPrefix(lastSend(t, d.client).Params["text"], "<b>Меню \"Профиль\"</b>"))
}

func TestProfileHandler_UnknownButtonAndStrayTextAreIgnored(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	openProfileMenu(t, d, h, me, myProfile)
	d.client.Reset()

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, "profile_unknown")))
	assert.Empty(t, d.client.Calls())

	stray := testutils.NewPrivateMessage(testUserID, "просто текст")
	require.True(t, d.dispatch(t, h, testutils.MessageUpdate(stray)))
	assert.Equal(t, []int64{stray.MessageId}, deletedMessageIDs(d.client), "stray text is deleted by the fallback")
	assert.Empty(t, d.client.CallsTo("sendMessage"))

	// The menu is still active
	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)))
	assert.Contains(t, lastSend(t, d.client).Params["text"], profileMenuEditHeader)
}

func TestProfileHandler_Cancel(t *testing.T) {
	tests := []struct {
		name   string
		open   func(t *testing.T, d *deps, h ext.Handler) int64
		update func() *gotgbot.Update
		answer bool
	}{
		{
			name:   "cancel button in the main menu",
			open:   func(t *testing.T, d *deps, h ext.Handler) int64 { return openProfileMenu(t, d, h, me, myProfile) },
			update: func() *gotgbot.Update { return testutils.CallbackUpdate(testUserID, constants.ProfileFullCancel) },
			answer: true,
		},
		{
			name: "cancel button while editing a field",
			open: func(t *testing.T, d *deps, h ext.Handler) int64 {
				return openFieldEditor(t, d, h, constants.ProfileEditLastnameCallback, me, myProfile)
			},
			update: func() *gotgbot.Update { return testutils.CallbackUpdate(testUserID, constants.ProfileFullCancel) },
			answer: true,
		},
		{
			name: "cancel button while searching",
			open: func(t *testing.T, d *deps, h ext.Handler) int64 {
				openProfileMenu(t, d, h, me, myProfile)
				require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileSearchProfileCallback)))
				return d.lastSentID(t, profileMenuSearchHeader)
			},
			update: func() *gotgbot.Update { return testutils.CallbackUpdate(testUserID, constants.ProfileFullCancel) },
			answer: true,
		},
		{
			name: "cancel command while editing bio",
			open: func(t *testing.T, d *deps, h ext.Handler) int64 {
				return openFieldEditor(t, d, h, constants.ProfileEditBioCallback, me, myProfile)
			},
			update: func() *gotgbot.Update { return privateText(testUserID, "/cancel") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			lastBotMessageID := tt.open(t, d, h)
			d.client.Reset()

			require.True(t, d.dispatch(t, h, tt.update()))

			if tt.answer {
				assert.Len(t, d.client.CallsTo("answerCallbackQuery"), 1)
			}
			assert.Equal(t, []string{profileSessionEndedText}, d.client.SentTexts())
			assert.Equal(t, testUserID, lastSend(t, d.client).Int64("chat_id"))
			assert.Equal(t, []int64{lastBotMessageID}, deletedMessageIDs(d.client))
			assert.False(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)),
				"the conversation is over")
		})
	}
}

// ---------- editing fields ----------

func TestProfileHandler_FieldEditorShowsCurrentValue(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		profile  testDBProfile
		header   string
		current  string
		ask      string
	}{
		{
			name:     "firstname",
			callback: constants.ProfileEditFirstnameCallback,
			profile:  myProfile,
			header:   profileMenuEditFirstnameHeader,
			current:  "Текущее значение: <code>Иван</code>",
			ask:      "Введи новое имя:",
		},
		{
			name:     "lastname",
			callback: constants.ProfileEditLastnameCallback,
			profile:  myProfile,
			header:   profileMenuEditLastnameHeader,
			current:  "Текущее значение: <code>Петров</code>",
			ask:      "Введи новую фамилию:",
		},
		{
			name:     "bio is HTML-escaped",
			callback: constants.ProfileEditBioCallback,
			profile:  testDBProfile{ID: 3, UserID: 7, Bio: "Люблю <b>Go</b> & Rust"},
			header:   profileMenuEditBioHeader,
			current:  "Текущее значение: <pre>Люблю &lt;b&gt;Go&lt;/b&gt; & Rust</pre>",
			ask:      "Введи обновлённую биографию (до 3900 символов):",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			menuID := openProfileMenu(t, d, h, me, tt.profile)
			d.client.Reset()
			d.expectUserWithProfile(me, tt.profile)

			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, tt.callback)))

			assert.Equal(t, []int64{menuID}, deletedMessageIDs(d.client))
			prompt := lastSend(t, d.client)
			assert.Equal(t, "<b>"+tt.header+"</b>\n\n"+tt.current+"\n\n"+tt.ask, prompt.Params["text"])
			assert.Equal(t, "HTML", prompt.Params["parse_mode"])
			assert.Equal(t, []string{constants.ProfileEditMyProfileCallback, constants.ProfileFullCancel}, callbackData(t, prompt))

			// "Back" returns to the edit menu
			d.client.Reset()
			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)))
			assert.Contains(t, lastSend(t, d.client).Params["text"], profileMenuEditHeader)
		})
	}
}

func TestProfileHandler_FieldEditorDatabaseErrors(t *testing.T) {
	tests := []struct {
		name    string
		expect  func(d *deps)
		wantErr string
	}{
		{
			name: "user",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByTgID).WithArgs(testUserID).WillReturnError(errors.New("db down"))
			},
			wantErr: "failed to get user in handleEditField",
		},
		{
			name: "profile",
			expect: func(d *deps) {
				d.expectUser(me)
				d.sql.ExpectQuery(selectProfileByUser).WithArgs(me.ID).WillReturnError(errors.New("db down"))
			},
			wantErr: "failed to get/create profile in handleEditField",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			openProfileMenu(t, d, h, me, myProfile)
			d.client.Reset()
			tt.expect(d)

			handled, err := testutils.Dispatch(d.bot, h, testutils.CallbackUpdate(testUserID, constants.ProfileEditBioCallback))

			require.True(t, handled)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, d.client.CallsTo("sendMessage"))
		})
	}
}

func TestProfileHandler_SaveNameFields(t *testing.T) {
	tests := []struct {
		name      string
		callback  string
		column    string
		header    string
		savedText string
	}{
		{name: "firstname", callback: constants.ProfileEditFirstnameCallback, column: "firstname",
			header: profileMenuEditFirstnameHeader, savedText: "✅ Имя сохранено!"},
		{name: "lastname", callback: constants.ProfileEditLastnameCallback, column: "lastname",
			header: profileMenuEditLastnameHeader, savedText: "✅ Фамилия сохранена!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			incomplete := testDBProfile{ID: 3, UserID: 7} // no bio, so nothing is published
			promptID := openFieldEditor(t, d, h, tt.callback, me, incomplete)
			d.client.Reset()

			d.expectUserWithProfile(me, incomplete)
			d.sql.ExpectExec(strings.ReplaceAll(updateUserFieldSQL, "%s", tt.column)).
				WithArgs("Alexander", me.ID).WillReturnResult(sqlmock.NewResult(0, 1))
			d.expectUserWithProfile(me, incomplete)
			value := testutils.NewPrivateMessage(testUserID, "Alexander")

			dispatchFieldValue(t, d, h, testutils.MessageUpdate(value))

			assert.ElementsMatch(t, []int64{promptID, value.MessageId}, deletedMessageIDs(d.client))
			assert.Equal(t, []string{"*" + tt.header + "*\n\n" + tt.savedText}, d.client.SentTexts(),
				"an incomplete profile is not published")
			saved := lastSend(t, d.client)
			assert.Equal(t, "HTML", saved.Params["parse_mode"])
			assert.Equal(t, []string{constants.ProfileEditMyProfileCallback, constants.ProfileFullCancel}, callbackData(t, saved))
			assert.Empty(t, d.client.CallsTo("editMessageText"))
		})
	}
}

func TestProfileHandler_TooLongNameIsRejected(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		column   string
		wantText string
	}{
		{name: "firstname", callback: constants.ProfileEditFirstnameCallback, column: "firstname",
			wantText: "*Профиль → Редактирование → Имя*\n\nИмя слишком длинное. Пожалуйста, введи более короткое имя (не более 30 символов):"},
		{name: "lastname", callback: constants.ProfileEditLastnameCallback, column: "lastname",
			wantText: "*Профиль → Редактирование → Фамилия*\n\nФамилия слишком длинная. Пожалуйста, введи более короткую фамилию (не более 30 символов):"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			incomplete := testDBProfile{ID: 3, UserID: 7}
			promptID := openFieldEditor(t, d, h, tt.callback, me, incomplete)
			d.client.Reset()
			tooLong := testutils.NewPrivateMessage(testUserID, strings.Repeat("a", 31))

			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(tooLong)))

			assert.ElementsMatch(t, []int64{promptID, tooLong.MessageId}, deletedMessageIDs(d.client))
			rejection := lastSend(t, d.client)
			assert.Equal(t, tt.wantText, rejection.Params["text"])
			assert.Equal(t, "Markdown", rejection.Params["parse_mode"])
			assert.Equal(t, []string{constants.ProfileEditMyProfileCallback, constants.ProfileFullCancel}, callbackData(t, rejection))

			// Still waiting for the value: exactly 30 characters are accepted
			d.client.Reset()
			d.expectUserWithProfile(me, incomplete)
			d.sql.ExpectExec(strings.ReplaceAll(updateUserFieldSQL, "%s", tt.column)).
				WithArgs(strings.Repeat("b", 30), me.ID).WillReturnResult(sqlmock.NewResult(0, 1))
			d.expectUserWithProfile(me, incomplete)
			dispatchFieldValue(t, d, h, privateText(testUserID, strings.Repeat("b", 30)))
			assert.Contains(t, lastSend(t, d.client).Params["text"], "сохранен")
		})
	}
}

func TestProfileHandler_SaveFieldDatabaseError(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		expect   func(d *deps)
		wantText string
		wantErr  string
	}{
		{
			name:     "firstname update fails",
			callback: constants.ProfileEditFirstnameCallback,
			expect: func(d *deps) {
				d.expectUserWithProfile(me, myProfile)
				d.sql.ExpectExec(strings.ReplaceAll(updateUserFieldSQL, "%s", "firstname")).
					WillReturnError(errors.New("db down"))
			},
			wantText: "*Профиль → Редактирование → Имя*\n\nПроизошла ошибка при сохранении имени.",
			wantErr:  "failed to save firstname",
		},
		{
			name:     "lastname user lookup fails",
			callback: constants.ProfileEditLastnameCallback,
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByTgID).WillReturnError(errors.New("db down"))
			},
			wantText: "*Профиль → Редактирование → Фамилия*\n\nПроизошла ошибка при сохранении фамилии.",
			wantErr:  "failed to save lastname",
		},
		{
			name:     "lastname profile lookup fails",
			callback: constants.ProfileEditLastnameCallback,
			expect: func(d *deps) {
				d.expectUser(me)
				d.sql.ExpectQuery(selectProfileByUser).WillReturnError(errors.New("db down"))
			},
			wantText: "*Профиль → Редактирование → Фамилия*\n\nПроизошла ошибка при сохранении фамилии.",
			wantErr:  "failed to save lastname",
		},
		{
			name:     "bio update fails",
			callback: constants.ProfileEditBioCallback,
			expect: func(d *deps) {
				d.expectUserWithProfile(me, myProfile)
				d.sql.ExpectExec(updateProfileBioSQL).WillReturnError(errors.New("db down"))
			},
			wantText: "*Профиль → Редактирование → О себе*\n\nПроизошла ошибка при сохранении биографии.",
			wantErr:  "failed to save bio",
		},
		{
			name:     "bio user lookup fails",
			callback: constants.ProfileEditBioCallback,
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByTgID).WillReturnError(errors.New("db down"))
			},
			wantText: "*Профиль → Редактирование → О себе*\n\nПроизошла ошибка при сохранении биографии.",
			wantErr:  "failed to save bio",
		},
		{
			name:     "bio profile lookup fails",
			callback: constants.ProfileEditBioCallback,
			expect: func(d *deps) {
				d.expectUser(me)
				d.sql.ExpectQuery(selectProfileByUser).WillReturnError(errors.New("db down"))
			},
			wantText: "*Профиль → Редактирование → О себе*\n\nПроизошла ошибка при сохранении биографии.",
			wantErr:  "failed to save bio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			openFieldEditor(t, d, h, tt.callback, me, myProfile)
			d.client.Reset()
			tt.expect(d)

			handled, err := testutils.Dispatch(d.bot, h, privateText(testUserID, "Value"))

			require.True(t, handled)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Equal(t, []string{tt.wantText}, d.client.SentTexts())
			reply := lastSend(t, d.client)
			assert.Equal(t, "Markdown", reply.Params["parse_mode"])
			assert.Contains(t, reply.Params["reply_parameters"], `"message_id":`)
			assert.Empty(t, d.client.CallsTo("deleteMessage"))
		})
	}
}

// ---------- bio and publishing ----------

func TestProfileHandler_TooLongBioIsRejected(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	promptID := openFieldEditor(t, d, h, constants.ProfileEditBioCallback, me, myProfile)
	d.client.Reset()
	// Emojis take two UTF-16 code units each, which is how Telegram measures text length
	tooLong := testutils.NewPrivateMessage(testUserID, strings.Repeat("😀", 1951))

	require.True(t, d.dispatch(t, h, testutils.MessageUpdate(tooLong)))

	assert.ElementsMatch(t, []int64{promptID, tooLong.MessageId}, deletedMessageIDs(d.client))
	rejection := lastSend(t, d.client)
	assert.Equal(t, "*Профиль → Редактирование → О себе*\n\nТекущая длина: 3902 символов\n\nПожалуйста, сократи до 3900 символов и пришли снова:",
		rejection.Params["text"])
	assert.Equal(t, "Markdown", rejection.Params["parse_mode"])
	assert.Equal(t, []string{constants.ProfileEditMyProfileCallback, constants.ProfileFullCancel}, callbackData(t, rejection))
}

func TestProfileHandler_BioPartsWithTheSameDateAreSkipped(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	openFieldEditor(t, d, h, constants.ProfileEditBioCallback, me, myProfile)

	firstPart := testutils.NewPrivateMessage(testUserID, strings.Repeat("a", 3901))
	firstPart.Date = 1700000100
	require.True(t, d.dispatch(t, h, testutils.MessageUpdate(firstPart)))
	d.client.Reset()

	// Telegram splits a very long text into several messages with the same date
	secondPart := testutils.NewPrivateMessage(testUserID, "хвост длинной биографии")
	secondPart.Date = firstPart.Date
	require.True(t, d.dispatch(t, h, testutils.MessageUpdate(secondPart)))

	assert.Equal(t, []int64{secondPart.MessageId}, deletedMessageIDs(d.client))
	assert.Empty(t, d.client.CallsTo("sendMessage"))
}

func TestProfileHandler_SaveBioPublishesNewProfile(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	unpublished := testDBProfile{ID: 3, UserID: 7, Bio: "Старая био"}
	promptID := openFieldEditor(t, d, h, constants.ProfileEditBioCallback, me, unpublished)
	d.client.Reset()

	updated := testDBProfile{ID: 3, UserID: 7, Bio: "Новая <био>"}
	publishedID := &captureArg{}
	d.expectUserWithProfile(me, unpublished)
	d.sql.ExpectExec(updateProfileBioSQL).WithArgs("Новая <био>", unpublished.ID).WillReturnResult(sqlmock.NewResult(0, 1))
	d.expectUserWithProfile(me, updated)
	d.sql.ExpectExec(updatePublishedIDSQL).WithArgs(publishedID, unpublished.ID).WillReturnResult(sqlmock.NewResult(0, 1))
	value := testutils.NewPrivateMessage(testUserID, "Новая <био>")

	dispatchFieldValue(t, d, h, testutils.MessageUpdate(value))

	publicText := formatters.FormatPublicProfileForMessage(dbUser(me), dbProfile(updated), false)
	assert.Contains(t, publicText, "Новая &lt;био&gt;")
	publication := sendWithText(t, d.client, publicText)
	assert.Equal(t, testFullSuperGroupID, publication.Int64("chat_id"))
	assert.Equal(t, int64(testIntroTopicID), publication.Int64("message_thread_id"))
	assert.Equal(t, "HTML", publication.Params["parse_mode"])
	assert.Contains(t, publication.Params["link_preview_options"], `"is_disabled":true`)
	assert.Equal(t, d.sentID(t, publicText), publishedID.Value(), "the new intro message id is stored")

	assert.ElementsMatch(t, []int64{promptID, value.MessageId}, deletedMessageIDs(d.client))
	saved := lastSend(t, d.client)
	assert.Equal(t, testUserID, saved.Int64("chat_id"))
	assert.True(t, strings.HasPrefix(saved.Params["text"], "*Профиль → Редактирование → О себе*\n\n✅ Биография сохранена!\n✅ Профиль <a href='"))
	assert.True(t, strings.HasSuffix(saved.Params["text"], "'>опубликован</a> на канале \"Интро\"."))
	assert.Empty(t, d.client.CallsTo("editMessageText"))
}

func TestProfileHandler_SaveBioUpdatesPublishedProfile(t *testing.T) {
	tests := []struct {
		name          string
		editError     string
		wantNewPost   bool
		wantStoredID  func(t *testing.T, d *deps, publicText string) int64
		wantLinkToOld bool
	}{
		{
			name:          "edit succeeds",
			wantStoredID:  func(*testing.T, *deps, string) int64 { return 555 },
			wantLinkToOld: true,
		},
		{
			name:          "content is unchanged",
			editError:     "Bad Request: message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message",
			wantStoredID:  func(*testing.T, *deps, string) int64 { return 555 },
			wantLinkToOld: true,
		},
		{
			name:        "published message is gone",
			editError:   "Bad Request: message to edit not found",
			wantNewPost: true,
			wantStoredID: func(t *testing.T, d *deps, publicText string) int64 {
				return d.sentID(t, publicText)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			published := testDBProfile{ID: 3, UserID: 7, Bio: "Старая био", PublishedID: int64(555)}
			openFieldEditor(t, d, h, constants.ProfileEditBioCallback, me, published)
			d.client.Reset()
			if tt.editError != "" {
				d.client.OnError("editMessageText", tt.editError)
			}

			updated := testDBProfile{ID: 3, UserID: 7, Bio: "Новая био", PublishedID: int64(555)}
			storedID := &captureArg{}
			d.expectUserWithProfile(me, published)
			d.sql.ExpectExec(updateProfileBioSQL).WithArgs("Новая био", published.ID).WillReturnResult(sqlmock.NewResult(0, 1))
			d.expectUserWithProfile(me, updated)
			d.sql.ExpectExec(updatePublishedIDSQL).WithArgs(storedID, published.ID).WillReturnResult(sqlmock.NewResult(0, 1))

			dispatchFieldValue(t, d, h, privateText(testUserID, "Новая био"))

			publicText := formatters.FormatPublicProfileForMessage(dbUser(me), dbProfile(updated), false)
			edit, ok := d.client.LastCallTo("editMessageText")
			require.True(t, ok)
			assert.Equal(t, testFullSuperGroupID, edit.Int64("chat_id"))
			assert.Equal(t, int64(555), edit.Int64("message_id"))
			assert.Equal(t, publicText, edit.Params["text"])
			assert.Equal(t, "HTML", edit.Params["parse_mode"])

			var groupPosts []string
			for _, c := range d.client.CallsTo("sendMessage") {
				if c.Int64("chat_id") == testFullSuperGroupID {
					groupPosts = append(groupPosts, c.Params["text"])
				}
			}
			if tt.wantNewPost {
				assert.Equal(t, []string{publicText}, groupPosts, "the profile is re-posted in the intro topic")
				post := sendWithText(t, d.client, publicText)
				assert.Equal(t, int64(testIntroTopicID), post.Int64("message_thread_id"))
			} else {
				assert.Empty(t, groupPosts)
			}
			assert.Equal(t, tt.wantStoredID(t, d, publicText), storedID.Value())

			saved := lastSend(t, d.client).Params["text"]
			assert.Contains(t, saved, "✅ Биография сохранена!")
			if tt.wantLinkToOld {
				assert.Contains(t, saved, "<a href='https://telegram.me/c/1234567890/13/555'>опубликован</a>")
			}
		})
	}
}

func TestProfileHandler_SaveBioWhenPublishingFails(t *testing.T) {
	tests := []struct {
		name   string
		expect func(d *deps)
		setup  func(d *deps)
	}{
		{
			name: "user lookup fails",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByTgID).WillReturnError(errors.New("db down"))
			},
		},
		{
			name: "profile lookup fails",
			expect: func(d *deps) {
				d.expectUser(me)
				d.sql.ExpectQuery(selectProfileByUser).WillReturnError(errors.New("db down"))
			},
		},
		{
			name: "posting to the intro topic fails",
			expect: func(d *deps) {
				d.expectUserWithProfile(me, testDBProfile{ID: 3, UserID: 7, Bio: "Новая био"})
			},
			setup: func(d *deps) {
				d.client.On("sendMessage", func(call testutils.APICall) (json.RawMessage, error) {
					if call.Int64("chat_id") == testFullSuperGroupID {
						return nil, &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: "Bad Request: TOPIC_DELETED"}
					}
					return d.sendMessageResponse(call)
				})
			},
		},
		{
			name: "re-posting after a failed edit fails",
			expect: func(d *deps) {
				d.expectUserWithProfile(me, testDBProfile{ID: 3, UserID: 7, Bio: "Новая био", PublishedID: int64(555)})
			},
			setup: func(d *deps) {
				d.client.OnError("editMessageText", "Bad Request: message to edit not found")
				d.client.On("sendMessage", func(call testutils.APICall) (json.RawMessage, error) {
					if call.Int64("chat_id") == testFullSuperGroupID {
						return nil, &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: "Bad Request: TOPIC_DELETED"}
					}
					return d.sendMessageResponse(call)
				})
			},
		},
		{
			name: "storing the published message id fails",
			expect: func(d *deps) {
				d.expectUserWithProfile(me, testDBProfile{ID: 3, UserID: 7, Bio: "Новая био"})
				d.sql.ExpectExec(updatePublishedIDSQL).WillReturnError(errors.New("db down"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			openFieldEditor(t, d, h, constants.ProfileEditBioCallback, me, myProfile)
			d.client.Reset()
			if tt.setup != nil {
				tt.setup(d)
			}
			d.expectUserWithProfile(me, myProfile)
			d.sql.ExpectExec(updateProfileBioSQL).WithArgs("Новая био", myProfile.ID).WillReturnResult(sqlmock.NewResult(0, 1))
			tt.expect(d)

			dispatchFieldValue(t, d, h, privateText(testUserID, "Новая био"))

			saved := lastSend(t, d.client)
			assert.Equal(t, testUserID, saved.Int64("chat_id"))
			assert.Equal(t, "*Профиль → Редактирование → О себе*\n\n✅ Биография сохранена!", saved.Params["text"],
				"the bio is saved even if the profile could not be published")
		})
	}
}

// ---------- searching profiles ----------

func openSearch(t *testing.T, d *deps, h ext.Handler) int64 {
	t.Helper()
	openProfileMenu(t, d, h, me, myProfile)
	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileSearchProfileCallback)))
	return d.lastSentID(t, profileMenuSearchHeader)
}

func TestProfileHandler_SearchPrompt(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	menuID := openProfileMenu(t, d, h, me, myProfile)
	d.client.Reset()

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileSearchProfileCallback)))

	assert.Equal(t, []int64{menuID}, deletedMessageIDs(d.client))
	prompt := lastSend(t, d.client)
	assert.Equal(t, "<b>Профиль → Поиск</b>\n\nВведи телеграм-ник пользователя <i>(с @ или без)</i>, либо его имя и фамилию <i>(через пробел)</i>:",
		prompt.Params["text"])
	assert.Equal(t, "HTML", prompt.Params["parse_mode"])
	assert.Equal(t, []string{constants.ProfileStartCallback, constants.ProfileFullCancel}, callbackData(t, prompt))

	// "Back" returns to the main menu
	searchID := d.lastSentID(t, profileMenuSearchHeader)
	d.client.Reset()
	d.expectUserWithProfile(me, myProfile)
	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileStartCallback)))
	assert.Equal(t, []int64{searchID}, deletedMessageIDs(d.client))
	assert.Contains(t, lastSend(t, d.client).Params["text"], profileMenuHeader)
}

func TestProfileHandler_SearchFindsProfile(t *testing.T) {
	other := testDBUser{ID: 8, TgID: 1010, Firstname: "Анна", Lastname: "Смирнова", Username: "annasmirnova"}

	tests := []struct {
		name     string
		query    string
		expect   func(d *deps)
		profile  testDBProfile
		wantLink bool
	}{
		{
			name:  "by username with @",
			query: "@annasmirnova",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WithArgs("annasmirnova").WillReturnRows(userRows(other))
			},
			profile:  testDBProfile{ID: 4, UserID: 8, Bio: "Data <science>", PublishedID: int64(777)},
			wantLink: true,
		},
		{
			name:  "by username without @",
			query: "annasmirnova",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WithArgs("annasmirnova").WillReturnRows(userRows(other))
			},
			profile: testDBProfile{ID: 4, UserID: 8, Bio: "Data science"},
		},
		{
			name:  "by first and last name",
			query: "Анна Смирнова",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WithArgs("Анна Смирнова").WillReturnRows(sqlmock.NewRows(userColumns))
				d.sql.ExpectQuery(selectUserByName).WithArgs("Анна", "Смирнова").WillReturnRows(userRows(other))
			},
			profile:  testDBProfile{ID: 4, UserID: 8, Bio: "Data science", PublishedID: int64(777)},
			wantLink: true,
		},
		{
			name:  "by first name and a compound last name",
			query: "Анна Смирнова Петрова",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WithArgs("Анна Смирнова Петрова").WillReturnRows(sqlmock.NewRows(userColumns))
				d.sql.ExpectQuery(selectUserByName).WithArgs("Анна", "Смирнова Петрова").WillReturnRows(userRows(other))
			},
			profile: testDBProfile{ID: 4, UserID: 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			searchID := openSearch(t, d, h)
			d.client.Reset()
			tt.expect(d)
			d.expectProfile(tt.profile)
			query := testutils.NewPrivateMessage(testUserID, tt.query)

			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(query)))

			assert.ElementsMatch(t, []int64{searchID, query.MessageId}, deletedMessageIDs(d.client))
			result := lastSend(t, d.client)
			want := "<b>Профиль → Поиск</b>\n\n" + formatters.FormatProfileView(dbUser(other), dbProfile(tt.profile), false)
			if tt.wantLink {
				want += "\n👉 <a href='https://telegram.me/c/1234567890/13/777'>Ссылка</a> на профиль в канале \"Интро\"."
			}
			assert.Equal(t, want, result.Params["text"])
			assert.Equal(t, "HTML", result.Params["parse_mode"])
			assert.Equal(t, []string{constants.ProfileStartCallback, constants.ProfileFullCancel}, callbackData(t, result))

			// The result is shown in the main menu state, so "back" opens the menu
			d.client.Reset()
			d.expectUserWithProfile(me, myProfile)
			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, constants.ProfileStartCallback)))
			assert.Contains(t, lastSend(t, d.client).Params["text"], profileMenuHeader)
		})
	}
}

func TestProfileHandler_SearchNotFound(t *testing.T) {
	d := newDeps(t)
	h := d.newProfileHandler()
	searchID := openSearch(t, d, h)
	d.client.Reset()
	d.sql.ExpectQuery(selectUserByUsername).WithArgs("nobody").WillReturnRows(sqlmock.NewRows(userColumns))
	d.sql.ExpectQuery(selectUserByName).WithArgs("nobody", "").WillReturnRows(sqlmock.NewRows(userColumns))
	query := testutils.NewPrivateMessage(testUserID, "@nobody")

	require.True(t, d.dispatch(t, h, testutils.MessageUpdate(query)))

	assert.ElementsMatch(t, []int64{searchID, query.MessageId}, deletedMessageIDs(d.client))
	notFound := lastSend(t, d.client)
	assert.Equal(t, "*Профиль → Поиск*\n\nПользователь *nobody* не найден.\n\nПопробуй ещё раз, прислав мне имя пользователя снова:",
		notFound.Params["text"])
	assert.Equal(t, "Markdown", notFound.Params["parse_mode"])
	assert.Equal(t, []string{constants.ProfileStartCallback, constants.ProfileFullCancel}, callbackData(t, notFound))

	// Still in search mode: the next query is processed
	d.client.Reset()
	other := testDBUser{ID: 8, TgID: 1010, Firstname: "Анна", Lastname: "Смирнова", Username: "anna"}
	d.sql.ExpectQuery(selectUserByUsername).WithArgs("anna").WillReturnRows(userRows(other))
	d.expectProfile(testDBProfile{ID: 4, UserID: 8, Bio: "bio"})
	require.True(t, d.dispatch(t, h, privateText(testUserID, "anna")))
	assert.Contains(t, lastSend(t, d.client).Params["text"], "Анна Смирнова")
	assert.Contains(t, deletedMessageIDs(d.client), d.lastSentID(t, "не найден"))
}

func TestProfileHandler_SearchDatabaseErrors(t *testing.T) {
	other := testDBUser{ID: 8, TgID: 1010, Firstname: "Анна", Lastname: "Смирнова", Username: "anna"}

	tests := []struct {
		name      string
		expect    func(d *deps)
		wantErr   string
		wantTexts []string
	}{
		{
			name: "username lookup fails",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WillReturnError(errors.New("db down"))
			},
			wantErr: "failed to get user in handleUsernameInput by username",
		},
		{
			name: "name lookup fails",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WillReturnRows(sqlmock.NewRows(userColumns))
				d.sql.ExpectQuery(selectUserByName).WillReturnError(errors.New("db down"))
			},
			wantErr: "failed to search user in handleUsernameInput by full name",
		},
		{
			name: "profile lookup fails",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WithArgs("anna").WillReturnRows(userRows(other))
				d.sql.ExpectQuery(selectProfileByUser).WithArgs(other.ID).WillReturnError(errors.New("db down"))
			},
			wantErr:   "failed to get profile in handleUsernameInput",
			wantTexts: []string{"Произошла ошибка при получении профиля."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			openSearch(t, d, h)
			d.client.Reset()
			tt.expect(d)

			handled, err := testutils.Dispatch(d.bot, h, privateText(testUserID, "anna"))

			require.True(t, handled)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Equal(t, tt.wantTexts, d.client.SentTexts())
		})
	}
}

func TestProfileHandler_SendFailuresAreReturned(t *testing.T) {
	incomplete := testDBProfile{ID: 3, UserID: 7}
	other := testDBUser{ID: 8, TgID: 1010, Firstname: "Анна", Lastname: "Смирнова", Username: "anna"}

	tests := []struct {
		name    string
		open    func(t *testing.T, d *deps, h ext.Handler)
		expect  func(d *deps)
		update  func() *gotgbot.Update
		wantErr string
	}{
		{
			name: "edit menu",
			open: func(t *testing.T, d *deps, h ext.Handler) { openProfileMenu(t, d, h, me, myProfile) },
			update: func() *gotgbot.Update {
				return testutils.CallbackUpdate(testUserID, constants.ProfileEditMyProfileCallback)
			},
			wantErr: "failed to send message in handleEditMyProfile",
		},
		{
			name: "search prompt",
			open: func(t *testing.T, d *deps, h ext.Handler) { openProfileMenu(t, d, h, me, myProfile) },
			update: func() *gotgbot.Update {
				return testutils.CallbackUpdate(testUserID, constants.ProfileSearchProfileCallback)
			},
			wantErr: "failed to send message in handleViewOtherProfile",
		},
		{
			name:    "field prompt",
			open:    func(t *testing.T, d *deps, h ext.Handler) { openProfileMenu(t, d, h, me, myProfile) },
			expect:  func(d *deps) { d.expectUserWithProfile(me, myProfile) },
			update:  func() *gotgbot.Update { return testutils.CallbackUpdate(testUserID, constants.ProfileEditBioCallback) },
			wantErr: "failed to send message in handleEditField",
		},
		{
			name: "search result",
			open: func(t *testing.T, d *deps, h ext.Handler) { openSearch(t, d, h) },
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WithArgs("anna").WillReturnRows(userRows(other))
				d.expectProfile(testDBProfile{ID: 4, UserID: 8})
			},
			update:  func() *gotgbot.Update { return privateText(testUserID, "anna") },
			wantErr: "failed to send message in handleUsernameInput",
		},
		{
			name: "search not found",
			open: func(t *testing.T, d *deps, h ext.Handler) { openSearch(t, d, h) },
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectUserByUsername).WillReturnRows(sqlmock.NewRows(userColumns))
				d.sql.ExpectQuery(selectUserByName).WillReturnRows(sqlmock.NewRows(userColumns))
			},
			update:  func() *gotgbot.Update { return privateText(testUserID, "nobody") },
			wantErr: "failed to send message in handleUsernameInput",
		},
		{
			name: "firstname saved",
			open: func(t *testing.T, d *deps, h ext.Handler) {
				openFieldEditor(t, d, h, constants.ProfileEditFirstnameCallback, me, incomplete)
			},
			expect: func(d *deps) {
				d.expectUserWithProfile(me, incomplete)
				d.sql.ExpectExec(strings.ReplaceAll(updateUserFieldSQL, "%s", "firstname")).WillReturnResult(sqlmock.NewResult(0, 1))
				d.expectUserWithProfile(me, incomplete)
			},
			update:  func() *gotgbot.Update { return privateText(testUserID, "Anna") },
			wantErr: "failed to send message in handleFirstnameInput",
		},
		{
			name: "lastname saved",
			open: func(t *testing.T, d *deps, h ext.Handler) {
				openFieldEditor(t, d, h, constants.ProfileEditLastnameCallback, me, incomplete)
			},
			expect: func(d *deps) {
				d.expectUserWithProfile(me, incomplete)
				d.sql.ExpectExec(strings.ReplaceAll(updateUserFieldSQL, "%s", "lastname")).WillReturnResult(sqlmock.NewResult(0, 1))
				d.expectUserWithProfile(me, incomplete)
			},
			update:  func() *gotgbot.Update { return privateText(testUserID, "Smirnova") },
			wantErr: "failed to send message in handleLastnameInput",
		},
		{
			name: "bio saved",
			open: func(t *testing.T, d *deps, h ext.Handler) {
				openFieldEditor(t, d, h, constants.ProfileEditBioCallback, me, myProfile)
			},
			expect: func(d *deps) {
				d.expectUserWithProfile(me, myProfile)
				d.sql.ExpectExec(updateProfileBioSQL).WillReturnResult(sqlmock.NewResult(0, 1))
				// complete profile: publishing to the intro topic fails as well and is ignored
				d.expectUserWithProfile(me, testDBProfile{ID: 3, UserID: 7, Bio: "bio"})
			},
			update:  func() *gotgbot.Update { return privateText(testUserID, "bio") },
			wantErr: "failed to send message in handleBioInput",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newProfileHandler()
			tt.open(t, d, h)
			if tt.expect != nil {
				tt.expect(d)
			}
			d.client.OnError("sendMessage", "Forbidden: bot was blocked by the user")

			handled, err := testutils.Dispatch(d.bot, h, tt.update())

			require.True(t, handled)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
