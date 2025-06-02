package privatehandlers

import (
	"errors"
	"testing"
	// "time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/models"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks (can be shared or defined per test file if structures differ)
type MockUserRepository struct {
	mock.Mock
}
func (m *MockUserRepository) GetUserByTgID(tgID int64) (*repositories.User, error) { // Assuming User is repositories.User
	args := m.Called(tgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repositories.User), args.Error(1)
}
// Add other methods if needed by other handlers, but not by this one specifically
func (m *MockUserRepository) GetByID(id int) (*repositories.User, error) { return nil, nil }
func (m *MockUserRepository) Create(tgID int64, firstname string, lastname string, username string) (int, error) { return 0, nil }


type MockPollRepoForAnswer struct { // Renamed to avoid conflict if used in same package test
	mock.Mock
}
func (m *MockPollRepoForAnswer) GetPollByTelegramPollID(telegramPollID string) (*models.WeeklyMeetingPoll, error) {
	args := m.Called(telegramPollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WeeklyMeetingPoll), args.Error(1)
}
func (m *MockPollRepoForAnswer) CreatePoll(poll models.WeeklyMeetingPoll) (int64, error) { return 0, nil }
func (m *MockPollRepoForAnswer) GetLatestPollForChat(chatID int64) (*models.WeeklyMeetingPoll, error) { return nil, nil }


type MockParticipantRepoForAnswer struct { // Renamed
	mock.Mock
}
func (m *MockParticipantRepoForAnswer) UpsertParticipant(participant models.WeeklyMeetingParticipant) error {
	args := m.Called(participant)
	return args.Error(0)
}
func (m *MockParticipantRepoForAnswer) RemoveParticipant(pollID int64, userID int64) error {
	args := m.Called(pollID, userID)
	return args.Error(0)
}
func (m *MockParticipantRepoForAnswer) GetParticipatingUsers(pollID int64) ([]repositories.User, error) { return nil, nil }
func (m *MockParticipantRepoForAnswer) GetParticipant(pollID int64, userID int64) (*models.WeeklyMeetingParticipant, error) { return nil, nil }


func TestPollAnswerHandler_HandleUpdate(t *testing.T) {
	mockCfg := &config.Config{} // Config not directly used in logic, but needed for constructor
	
	tgUser := &gotgbot.User{Id: 101, Username: "testuser", FirstName: "Test"}
	internalUser := &repositories.User{ID: 1, TgID: tgUser.Id, Firstname: "Test"}
	
	trackedPollID := "trackedPoll123"
	retrievedPoll := &models.WeeklyMeetingPoll{ID: 5, TelegramPollID: trackedPollID, ChatID: -100}

	tests := []struct {
		name                    string
		pollAnswer              *gotgbot.PollAnswer
		mockUserRepoSetup       func(m *MockUserRepository)
		mockPollRepoSetup       func(m *MockPollRepoForAnswer)
		mockParticipantRepoSetup func(m *MockParticipantRepoForAnswer)
		expectedError           bool // If the handler function itself should return an error (usually no)
	}{
		{
			name:       "Nil PollAnswer", // Should ideally not happen with filter
			pollAnswer: nil,
		},
		{
			name: "UserRepo GetUserByTgID returns error",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{0}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(nil, errors.New("db user error")).Once()
			},
		},
		{
			name: "UserRepo GetUserByTgID returns nil (user not found)",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{0}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(nil, nil).Once()
			},
		},
		{
			name: "PollRepo GetPollByTelegramPollID returns error",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{0}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(internalUser, nil).Once()
			},
			mockPollRepoSetup: func(m *MockPollRepoForAnswer) {
				m.On("GetPollByTelegramPollID", trackedPollID).Return(nil, errors.New("db poll error")).Once()
			},
		},
		{
			name: "PollRepo GetPollByTelegramPollID returns nil (poll not tracked)",
			pollAnswer: &gotgbot.PollAnswer{PollId: "untrackedPoll", User: *tgUser, OptionIds: []int32{0}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(internalUser, nil).Once()
			},
			mockPollRepoSetup: func(m *MockPollRepoForAnswer) {
				m.On("GetPollByTelegramPollID", "untrackedPoll").Return(nil, nil).Once()
			},
		},
		{
			name: "Vote Retraction - Success",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{}}, // Empty options
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(internalUser, nil).Once()
			},
			mockPollRepoSetup: func(m *MockPollRepoForAnswer) {
				m.On("GetPollByTelegramPollID", trackedPollID).Return(retrievedPoll, nil).Once()
			},
			mockParticipantRepoSetup: func(m *MockParticipantRepoForAnswer) {
				m.On("RemoveParticipant", retrievedPoll.ID, internalUser.ID).Return(nil).Once()
			},
		},
		{
			name: "Vote Retraction - Error",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(internalUser, nil).Once()
			},
			mockPollRepoSetup: func(m *MockPollRepoForAnswer) {
				m.On("GetPollByTelegramPollID", trackedPollID).Return(retrievedPoll, nil).Once()
			},
			mockParticipantRepoSetup: func(m *MockParticipantRepoForAnswer) {
				m.On("RemoveParticipant", retrievedPoll.ID, internalUser.ID).Return(errors.New("db remove error")).Once()
			},
		},
		{
			name: "New 'Yes' Vote (Option 0) - Success",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{0}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(internalUser, nil).Once()
			},
			mockPollRepoSetup: func(m *MockPollRepoForAnswer) {
				m.On("GetPollByTelegramPollID", trackedPollID).Return(retrievedPoll, nil).Once()
			},
			mockParticipantRepoSetup: func(m *MockParticipantRepoForAnswer) {
				expectedParticipant := models.WeeklyMeetingParticipant{
					PollID:          retrievedPoll.ID,
					UserID:          internalUser.ID,
					IsParticipating: true,
				}
				m.On("UpsertParticipant", expectedParticipant).Return(nil).Once()
			},
		},
		{
			name: "New 'No' Vote (Option 1) - Success",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{1}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(internalUser, nil).Once()
			},
			mockPollRepoSetup: func(m *MockPollRepoForAnswer) {
				m.On("GetPollByTelegramPollID", trackedPollID).Return(retrievedPoll, nil).Once()
			},
			mockParticipantRepoSetup: func(m *MockParticipantRepoForAnswer) {
				expectedParticipant := models.WeeklyMeetingParticipant{
					PollID:          retrievedPoll.ID,
					UserID:          internalUser.ID,
					IsParticipating: false,
				}
				m.On("UpsertParticipant", expectedParticipant).Return(nil).Once()
			},
		},
		{
			name: "UpsertParticipant - Error",
			pollAnswer: &gotgbot.PollAnswer{PollId: trackedPollID, User: *tgUser, OptionIds: []int32{0}},
			mockUserRepoSetup: func(m *MockUserRepository) {
				m.On("GetUserByTgID", tgUser.Id).Return(internalUser, nil).Once()
			},
			mockPollRepoSetup: func(m *MockPollRepoForAnswer) {
				m.On("GetPollByTelegramPollID", trackedPollID).Return(retrievedPoll, nil).Once()
			},
			mockParticipantRepoSetup: func(m *MockParticipantRepoForAnswer) {
				m.On("UpsertParticipant", mock.AnythingOfType("models.WeeklyMeetingParticipant")).Return(errors.New("db upsert error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(MockUserRepository)
			mockPollRepo := new(MockPollRepoForAnswer)
			mockParticipantRepo := new(MockParticipantRepoForAnswer)

			if tt.mockUserRepoSetup != nil {
				tt.mockUserRepoSetup(mockUserRepo)
			}
			if tt.mockPollRepoSetup != nil {
				tt.mockPollRepoSetup(mockPollRepo)
			}
			if tt.mockParticipantRepoSetup != nil {
				tt.mockParticipantRepoSetup(mockParticipantRepo)
			}
			
			// Create handler instance using constructor for full coverage, though it returns ext.Handler
			// For testing the method directly, we can cast or use the struct.
			handlerStruct := &PollAnswerHandler{
				config: mockCfg,
				userRepo: mockUserRepo,
				pollRepo: mockPollRepo,
				participantRepo: mockParticipantRepo,
			}
			// handler := NewPollAnswerHandler(mockCfg, mockUserRepo, mockPollRepo, mockParticipantRepo)


			ctx := &ext.Context{PollAnswer: tt.pollAnswer}
			
			err := handlerStruct.handleUpdate(nil, ctx) // Bot can be nil if not used by handler method

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err) // Handler logs errors and returns nil
			}

			mockUserRepo.AssertExpectations(t)
			mockPollRepo.AssertExpectations(t)
			mockParticipantRepo.AssertExpectations(t)
		})
	}
}
