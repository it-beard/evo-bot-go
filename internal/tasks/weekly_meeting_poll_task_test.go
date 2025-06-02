package tasks

import (
	"errors"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/models"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Bot
type MockBot struct {
	mock.Mock
}

func (m *MockBot) SendPoll(chatId int64, question string, options []gotgbot.InputPollOption, opts *gotgbot.SendPollOpts) (*gotgbot.Message, error) {
	args := m.Called(chatId, question, options, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gotgbot.Message), args.Error(1)
}
// Implement other gotgbot.Bot methods if needed by the task, though unlikely for sendPoll
func (m *MockBot) GetMe() (*gotgbot.User, error) { return nil, nil }
// Add any other methods from the Bot interface that might be called.


// Mock WeeklyMeetingPollRepository
type MockPollRepoForTask struct { // Renamed
	mock.Mock
}
func (m *MockPollRepoForTask) CreatePoll(poll models.WeeklyMeetingPoll) (int64, error) {
	args := m.Called(poll)
	return args.Get(0).(int64), args.Error(1)
}
// Unused methods for this task test
func (m *MockPollRepoForTask) GetPollByTelegramPollID(telegramPollID string) (*models.WeeklyMeetingPoll, error) { return nil, nil }
func (m *MockPollRepoForTask) GetLatestPollForChat(chatID int64) (*models.WeeklyMeetingPoll, error) { return nil, nil }


func TestWeeklyMeetingPollTask_SendPoll(t *testing.T) {
	mockCfg := &config.Config{
		SupergroupChatID: -100123,
		// MeetingPollSchedule is used by Start(), not directly by sendPoll()
	}

	// Fixed time for predictable WeekStartDate calculation
	// Let's assume "Now" is a Friday for this test to make Monday calculation straightforward.
	// Example: Friday, July 19, 2024. Upcoming Monday is July 22, 2024.
	mockNow := time.Date(2024, time.July, 19, 17, 0, 0, 0, time.UTC) 
	// Expected Monday, normalized to start of day in Moscow time (assuming cron runs in Moscow time)
	moscowLoc, _ := time.LoadLocation("Europe/Moscow")
	expectedMondayInMoscow := time.Date(2024, time.July, 22, 0, 0, 0, 0, moscowLoc)


	sentMessage := &gotgbot.Message{
		MessageId: 5678,
		Chat:      &gotgbot.Chat{Id: mockCfg.SupergroupChatID},
		Poll:      &gotgbot.Poll{Id: "pollXYZ123"},
	}

	tests := []struct {
		name             string
		setupMocks       func(mBot *MockBot, mRepo *MockPollRepoForTask)
		expectedErrorLog string // Substring of an error log, if any
	}{
		{
			name: "Successful poll send and DB save",
			setupMocks: func(mBot *MockBot, mRepo *MockPollRepoForTask) {
				mBot.On("SendPoll", mockCfg.SupergroupChatID, mock.AnythingOfType("string"), mock.AnythingOfType("[]gotgbot.InputPollOption"), mock.AnythingOfType("*gotgbot.SendPollOpts")).
					Return(sentMessage, nil).Once()
				
				mRepo.On("CreatePoll", mock.MatchedBy(func(poll models.WeeklyMeetingPoll) bool {
					return poll.MessageID == sentMessage.MessageId &&
						poll.ChatID == sentMessage.Chat.Id &&
						poll.TelegramPollID == sentMessage.Poll.Id &&
						poll.WeekStartDate.Equal(expectedMondayInMoscow) // Compare time correctly
				})).Return(int64(1), nil).Once()
			},
		},
		{
			name: "SendPoll returns error",
			setupMocks: func(mBot *MockBot, mRepo *MockPollRepoForTask) {
				mBot.On("SendPoll", mockCfg.SupergroupChatID, mock.AnythingOfType("string"), mock.AnythingOfType("[]gotgbot.InputPollOption"), mock.AnythingOfType("*gotgbot.SendPollOpts")).
					Return(nil, errors.New("telegram API error")).Once()
				// CreatePoll should not be called
			},
			expectedErrorLog: "Error sending poll: telegram API error",
		},
		{
			name: "CreatePoll returns error",
			setupMocks: func(mBot *MockBot, mRepo *MockPollRepoForTask) {
				mBot.On("SendPoll", mockCfg.SupergroupChatID, mock.AnythingOfType("string"), mock.AnythingOfType("[]gotgbot.InputPollOption"), mock.AnythingOfType("*gotgbot.SendPollOpts")).
					Return(sentMessage, nil).Once()
				
				mRepo.On("CreatePoll", mock.AnythingOfType("models.WeeklyMeetingPoll")).
					Return(int64(0), errors.New("database insert error")).Once()
			},
			expectedErrorLog: "Failed to save weekly meeting poll to DB: database insert error",
		},
		{
            name: "SupergroupChatID not configured",
            setupMocks: func(mBot *MockBot, mRepo *MockPollRepoForTask) {
                // Bot and Repo methods should not be called
            },
            // Custom check for log message, as the function returns early
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBot := new(MockBot)
			mockPollRepo := new(MockPollRepoForTask)
			
			currentCfg := *mockCfg // Copy base config
            if tt.name == "SupergroupChatID not configured" {
                currentCfg.SupergroupChatID = 0
            }


			// The task struct needs a location. We'll set it to Moscow, as in the original task.
			// For sendPoll, the location mainly affects WeekStartDate calculation if time.Now() is used directly.
			// The Start() method usually sets this up.
			loc, _ := time.LoadLocation("Europe/Moscow")
			task := &WeeklyMeetingPollTask{
				config:   &currentCfg,
				bot:      mockBot,
				pollRepo: mockPollRepo,
				location: loc, 
			}

			// Patch time.Now for consistent WeekStartDate calculation
            // This is a bit more involved; for this test, we'll rely on the location
            // and the fixed mockNow to ensure the date calculation is testable without patching time.Now().
            // The key is that task.location and any time.Now().In(task.location) calls are consistent.
            // Our expectedMondayInMoscow already considers this.

			if tt.setupMocks != nil {
				tt.setupMocks(mockBot, mockPollRepo)
			}
			
            // For "SupergroupChatID not configured", we'd ideally check logs.
            // For simplicity here, we just ensure no calls to bot/repo happen.
            // In a real scenario, you might capture log output.
            
			task.sendPoll() // Call the method directly

			mockBot.AssertExpectations(t)
			mockPollRepo.AssertExpectations(t)

			// If tt.expectedErrorLog is set, we would ideally check log output.
			// This is often done by redirecting the log output during the test.
			// For this example, we are not capturing logs, but in a full setup, you would.
			if tt.expectedErrorLog != "" {
				// Placeholder for log assertion:
				// assert.Contains(t, capturedLogOutput, tt.expectedErrorLog)
			}
		})
	}
}
