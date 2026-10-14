package buttons

import (
	"encoding/json"
	"testing"

	"evo-bot-go/internal/constants"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type btn struct {
	text string
	data string
}

// keyboard builds the expected markup from rows of (text, callback data) pairs.
func keyboard(rows ...[]btn) gotgbot.InlineKeyboardMarkup {
	markup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: make([][]gotgbot.InlineKeyboardButton, len(rows))}
	for i, row := range rows {
		markup.InlineKeyboard[i] = make([]gotgbot.InlineKeyboardButton, len(row))
		for j, b := range row {
			markup.InlineKeyboard[i][j] = gotgbot.InlineKeyboardButton{Text: b.text, CallbackData: b.data}
		}
	}
	return markup
}

func row(buttons ...btn) []btn {
	return buttons
}

// assertValidCallbackData checks Telegram constraints for every button: non-empty text and
// callback data of 1-64 bytes.
func assertValidCallbackData(t *testing.T, markup gotgbot.InlineKeyboardMarkup) {
	t.Helper()
	require.NotEmpty(t, markup.InlineKeyboard)
	for i, r := range markup.InlineKeyboard {
		require.NotEmpty(t, r, "row %d is empty", i)
		for _, b := range r {
			assert.NotEmpty(t, b.Text)
			assert.NotEmpty(t, b.CallbackData, "button %q has no callback data", b.Text)
			assert.LessOrEqual(t, len(b.CallbackData), 64, "callback data %q exceeds Telegram's 64-byte limit", b.CallbackData)
		}
	}
}

func TestGeneralButtons(t *testing.T) {
	tests := []struct {
		name     string
		markup   gotgbot.InlineKeyboardMarkup
		expected gotgbot.InlineKeyboardMarkup
	}{
		{
			name:     "CancelButton",
			markup:   CancelButton("cancel_cb"),
			expected: keyboard(row(btn{"❌ Отмена", "cancel_cb"})),
		},
		{
			name:     "ConfirmButton",
			markup:   ConfirmButton("confirm_cb"),
			expected: keyboard(row(btn{"✅ Подтвердить", "confirm_cb"})),
		},
		{
			name:     "ConfirmAndCancelButton",
			markup:   ConfirmAndCancelButton("yes_cb", "no_cb"),
			expected: keyboard(row(btn{"✅ Подтвердить", "yes_cb"}, btn{"❌ Отмена", "no_cb"})),
		},
		{
			name:     "BackAndCancelButton",
			markup:   BackAndCancelButton("back_cb", "cancel_cb"),
			expected: keyboard(row(btn{"◀️ Назад", "back_cb"}, btn{"❌ Отмена", "cancel_cb"})),
		},
		{
			name:   "SearchTypeSelectionButton",
			markup: SearchTypeSelectionButton("fast_cb", "deep_cb", "cancel_cb"),
			expected: keyboard(
				row(btn{"⚡ Быстрый", "fast_cb"}, btn{"🔍 Глубокий", "deep_cb"}),
				row(btn{"❌ Отмена", "cancel_cb"}),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.markup)
			assertValidCallbackData(t, tt.markup)
		})
	}
}

func TestProfileHandlerButtons(t *testing.T) {
	const back = "profile_back_cb"

	tests := []struct {
		name     string
		markup   gotgbot.InlineKeyboardMarkup
		expected gotgbot.InlineKeyboardMarkup
	}{
		{
			name:   "ProfileMainButtons",
			markup: ProfileMainButtons(),
			expected: keyboard(
				row(btn{"✏️ Редактировать", constants.ProfileEditMyProfileCallback}),
				row(btn{"🔎 Поиск профиля по имени/нику", constants.ProfileSearchProfileCallback}),
				row(btn{"❌ Отмена", constants.ProfileFullCancel}),
			),
		},
		{
			name:   "ProfileEditBackCancelButtons",
			markup: ProfileEditBackCancelButtons(back),
			expected: keyboard(row(
				btn{"◀️ Назад", back},
				btn{"✏️ Редактировать", constants.ProfileEditMyProfileCallback},
				btn{"❌ Отмена", constants.ProfileFullCancel},
			)),
		},
		{
			name:   "ProfileBackCancelButtons",
			markup: ProfileBackCancelButtons(back),
			expected: keyboard(row(
				btn{"◀️ Назад", back},
				btn{"❌ Отмена", constants.ProfileFullCancel},
			)),
		},
		{
			name:   "ProfileEditButtons",
			markup: ProfileEditButtons(back),
			expected: keyboard(
				row(
					btn{"👤 Имя", constants.ProfileEditFirstnameCallback},
					btn{"👤 Фамилия", constants.ProfileEditLastnameCallback},
					btn{"📝 Био", constants.ProfileEditBioCallback},
				),
				row(
					btn{"◀️ Назад", back},
					btn{"❌ Отмена", constants.ProfileFullCancel},
				),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.markup)
			assertValidCallbackData(t, tt.markup)
		})
	}
}

func TestProfilesManagerHandlerButtons(t *testing.T) {
	const back = "admin_back_cb"

	tests := []struct {
		name     string
		markup   gotgbot.InlineKeyboardMarkup
		expected gotgbot.InlineKeyboardMarkup
	}{
		{
			name:   "ProfilesBackCancelButtons",
			markup: ProfilesBackCancelButtons(back),
			expected: keyboard(row(
				btn{"◀️ Назад", back},
				btn{"❌ Отмена", constants.AdminProfilesCancelCallback},
			)),
		},
		{
			name:   "ProfilesBackStartCancelButtons",
			markup: ProfilesBackStartCancelButtons(back),
			expected: keyboard(row(
				btn{"◀️ Назад", back},
				btn{"⏪ Старт", constants.AdminProfilesStartCallback},
				btn{"❌ Отмена", constants.AdminProfilesCancelCallback},
			)),
		},
		{
			name:   "ProfilesCoffeeBanButtons with ban",
			markup: ProfilesCoffeeBanButtons(back, true),
			expected: keyboard(
				row(btn{"✅ Разрешить", constants.AdminProfilesToggleCoffeeBanCallback}),
				row(btn{"◀️ Назад", back}, btn{"❌ Отмена", constants.AdminProfilesCancelCallback}),
			),
		},
		{
			name:   "ProfilesCoffeeBanButtons without ban",
			markup: ProfilesCoffeeBanButtons(back, false),
			expected: keyboard(
				row(btn{"❌ Запретить", constants.AdminProfilesToggleCoffeeBanCallback}),
				row(btn{"◀️ Назад", back}, btn{"❌ Отмена", constants.AdminProfilesCancelCallback}),
			),
		},
		{
			name:   "ProfilesEditMenuButtons",
			markup: ProfilesEditMenuButtons(back),
			expected: keyboard(
				row(
					btn{"👤 Имя", constants.AdminProfilesEditFirstnameCallback},
					btn{"👤 Фамилия", constants.AdminProfilesEditLastnameCallback},
					btn{"👤 Username", constants.AdminProfilesEditUsernameCallback},
				),
				row(
					btn{"📝 О себе", constants.AdminProfilesEditBioCallback},
					btn{"☕️ Кофе?", constants.AdminProfilesEditCoffeeBanCallback},
				),
				row(
					btn{"📢 Го! (+ превью)", constants.AdminProfilesPublishCallback},
					btn{"📢 Го! (- превью)", constants.AdminProfilesPublishNoPreviewCallback},
				),
				row(
					btn{"◀️ Назад", back},
					btn{"❌ Отмена", constants.AdminProfilesCancelCallback},
				),
			),
		},
		{
			name:   "ProfilesMainMenuButtons",
			markup: ProfilesMainMenuButtons(),
			expected: keyboard(
				row(btn{"📝 Поиск по Telegram Username", constants.AdminProfilesSearchByUsernameCallback}),
				row(btn{"🔍 Поиск по Telegram ID", constants.AdminProfilesSearchByTelegramIDCallback}),
				row(btn{"🔍 Поиск по имени и фамилии", constants.AdminProfilesSearchByFullNameCallback}),
				row(btn{"➕ Создать профиль (через реплай)", constants.AdminProfilesCreateByForwardedMessageCallback}),
				row(btn{"🆔 Создать профиль по TelegramID", constants.AdminProfilesCreateByTelegramIDCallback}),
				row(btn{"❌ Отмена", constants.AdminProfilesCancelCallback}),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.markup)
			assertValidCallbackData(t, tt.markup)
		})
	}
}

func TestProfilesCoffeeBanButtons_ToggleTextReflectsState(t *testing.T) {
	banned := ProfilesCoffeeBanButtons("b", true).InlineKeyboard[0][0]
	allowed := ProfilesCoffeeBanButtons("b", false).InlineKeyboard[0][0]

	assert.NotEqual(t, banned.Text, allowed.Text)
	assert.Equal(t, banned.CallbackData, allowed.CallbackData, "toggle uses the same callback in both states")
}

func TestButtons_ReturnIndependentMarkups(t *testing.T) {
	first := ProfileMainButtons()
	first.InlineKeyboard[0][0].Text = "mutated"

	second := ProfileMainButtons()

	assert.Equal(t, "✏️ Редактировать", second.InlineKeyboard[0][0].Text)
}

func TestButtons_SerializeAsTelegramInlineKeyboard(t *testing.T) {
	raw, err := json.Marshal(ConfirmAndCancelButton("yes", "no"))
	require.NoError(t, err)

	var decoded struct {
		InlineKeyboard [][]map[string]any `json:"inline_keyboard"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	require.Len(t, decoded.InlineKeyboard, 1)
	require.Len(t, decoded.InlineKeyboard[0], 2)
	assert.Equal(t, "✅ Подтвердить", decoded.InlineKeyboard[0][0]["text"])
	assert.Equal(t, "yes", decoded.InlineKeyboard[0][0]["callback_data"])
	assert.Equal(t, "❌ Отмена", decoded.InlineKeyboard[0][1]["text"])
	assert.Equal(t, "no", decoded.InlineKeyboard[0][1]["callback_data"])
}
