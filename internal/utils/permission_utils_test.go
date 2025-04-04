package utils

import (
	"errors"
	"testing"

	"evo-bot-go/internal/config"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
)

// mockBot is a mock implementation of the bot API needed for these tests.
type mockBot struct {
	GetChatMemberFunc func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error)
}

// GetChatMember implements the required method for the mock.
func (m *mockBot) GetChatMember(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
	if m.GetChatMemberFunc != nil {
		return m.GetChatMemberFunc(chatId, userId, opts)
	}
	return nil, errors.New("GetChatMemberFunc not implemented")
}

// newMockChatMember returns a concrete gotgbot.ChatMember type based on status.
func newMockChatMember(status string) gotgbot.ChatMember {
	user := gotgbot.User{Id: 123}
	switch status {
	case "creator":
		// The type itself implies the status.
		return &gotgbot.ChatMemberOwner{User: user}
	case "administrator":
		return &gotgbot.ChatMemberAdministrator{User: user}
	case "member":
		return &gotgbot.ChatMemberMember{User: user}
	case "restricted":
		// Need to provide required fields even if not used by GetStatus.
		return &gotgbot.ChatMemberRestricted{User: user}
	case "left":
		return &gotgbot.ChatMemberLeft{User: user}
	case "kicked":
		// Need to provide required fields even if not used by GetStatus.
		return &gotgbot.ChatMemberBanned{User: user}
	default:
		// Default to member or handle error appropriately.
		return &gotgbot.ChatMemberMember{User: user}
	}
}

func TestIsUserClubMember_RegularMember(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("member"), nil
		},
	}

	result := IsUserClubMember(bot, testUserID, mockConfig)
	assert.True(t, result, "Regular member should be considered a club member")
}

func TestIsUserClubMember_Administrator(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("administrator"), nil
		},
	}

	result := IsUserClubMember(bot, testUserID, mockConfig)
	assert.True(t, result, "Administrator should be considered a club member")
}

func TestIsUserClubMember_Creator(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("creator"), nil
		},
	}

	result := IsUserClubMember(bot, testUserID, mockConfig)
	assert.True(t, result, "Creator/owner should be considered a club member")
}

func TestIsUserClubMember_Restricted(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("restricted"), nil
		},
	}

	result := IsUserClubMember(bot, testUserID, mockConfig)
	assert.True(t, result, "Restricted user should still be considered a club member")
}

func TestIsUserClubMember_Left(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("left"), nil
		},
	}

	result := IsUserClubMember(bot, testUserID, mockConfig)
	assert.False(t, result, "User who left chat should not be considered a club member")
}

func TestIsUserClubMember_Kicked(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("kicked"), nil
		},
	}

	result := IsUserClubMember(bot, testUserID, mockConfig)
	assert.False(t, result, "Kicked/banned user should not be considered a club member")
}

func TestIsUserClubMember_Error(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return nil, assert.AnError
		},
	}

	result := IsUserClubMember(bot, testUserID, mockConfig)
	assert.False(t, result, "API error should result in user not being considered a club member")
}

func TestIsUserAdminOrCreator_Administrator(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("administrator"), nil
		},
	}

	result := IsUserAdminOrCreator(bot, testUserID, mockConfig)
	assert.True(t, result, "Administrator should have admin privileges")
}

func TestIsUserAdminOrCreator_Creator(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("creator"), nil
		},
	}

	result := IsUserAdminOrCreator(bot, testUserID, mockConfig)
	assert.True(t, result, "Creator/owner should have admin privileges")
}

func TestIsUserAdminOrCreator_RegularMember(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("member"), nil
		},
	}

	result := IsUserAdminOrCreator(bot, testUserID, mockConfig)
	assert.False(t, result, "Regular member should not have admin privileges")
}

func TestIsUserAdminOrCreator_Restricted(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("restricted"), nil
		},
	}

	result := IsUserAdminOrCreator(bot, testUserID, mockConfig)
	assert.False(t, result, "Restricted user should not have admin privileges")
}

func TestIsUserAdminOrCreator_Left(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("left"), nil
		},
	}

	result := IsUserAdminOrCreator(bot, testUserID, mockConfig)
	assert.False(t, result, "User who left chat should not have admin privileges")
}

func TestIsUserAdminOrCreator_Kicked(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return newMockChatMember("kicked"), nil
		},
	}

	result := IsUserAdminOrCreator(bot, testUserID, mockConfig)
	assert.False(t, result, "Kicked/banned user should not have admin privileges")
}

func TestIsUserAdminOrCreator_Error(t *testing.T) {
	mockConfig := &config.Config{SuperGroupChatID: -1001234567890}
	var testUserID int64 = 12345

	bot := &mockBot{
		GetChatMemberFunc: func(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
			return nil, assert.AnError
		},
	}

	result := IsUserAdminOrCreator(bot, testUserID, mockConfig)
	assert.False(t, result, "API error should result in user not having admin privileges")
}
