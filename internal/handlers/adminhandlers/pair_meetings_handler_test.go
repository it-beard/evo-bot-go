package adminhandlers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/models"
	"evo-bot-go/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock services and repositories
type MockPermissionsService struct {
	mock.Mock
	services.PermissionsService // Embed for interface satisfaction
}

func (m *MockPermissionsService) IsAdmin(userID int64) bool {
	args := m.Called(userID)
	return args.Bool(0)
}

type MockMessageSenderService struct {
	mock.Mock
	services.MessageSenderService
}

func (m *MockMessageSenderService) SendMessage(chatID int64, text string, opts *services.SendMessageOpts) (*gotgbot.Message, error) {
	args := m.Called(chatID, text, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gotgbot.Message), args.Error(1)
}

func (m *MockMessageSenderService) Reply(msg *gotgbot.Message, text string, opts *services.SendReplyOpts) (*gotgbot.Message, error) {
	args := m.Called(msg, text, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gotgbot.Message), args.Error(1)
}


type MockWeeklyMeetingPollRepository struct {
	mock.Mock
}

func (m *MockWeeklyMeetingPollRepository) GetLatestPollForChat(chatID int64) (*models.WeeklyMeetingPoll, error) {
	args := m.Called(chatID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WeeklyMeetingPoll), args.Error(1)
}
// Unused methods for this handler test
func (m *MockWeeklyMeetingPollRepository) CreatePoll(poll models.WeeklyMeetingPoll) (int64, error) { return 0, nil }
func (m *MockWeeklyMeetingPollRepository) GetPollByTelegramPollID(telegramPollID string) (*models.WeeklyMeetingPoll, error) { return nil, nil }


type MockWeeklyMeetingParticipantRepository struct {
	mock.Mock
}
func (m *MockWeeklyMeetingParticipantRepository) GetParticipatingUsers(pollID int64) ([]repositories.User, error) {
	args := m.Called(pollID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repositories.User), args.Error(1)
}
// Unused methods for this handler test
func (m *MockWeeklyMeetingParticipantRepository) UpsertParticipant(participant models.WeeklyMeetingParticipant) error { return nil }
func (m *MockWeeklyMeetingParticipantRepository) RemoveParticipant(pollID int64, userID int64) error { return nil }
func (m *MockWeeklyMeetingParticipantRepository) GetParticipant(pollID int64, userID int64) (*models.WeeklyMeetingParticipant, error) { return nil, nil }


func TestPairMeetingsHandler_HandleCommand(t *testing.T) {
	mockCfg := &config.Config{SupergroupChatID: -100123}
	adminUser := &gotgbot.User{Id: 123, Username: "admin"}
	regularUser := &gotgbot.User{Id: 456, Username: "user"}
	
	baseTime := time.Date(2024, time.July, 22, 0, 0, 0, 0, time.UTC) // A Monday

	mockPoll := &models.WeeklyMeetingPoll{
		ID:            1,
		ChatID:        mockCfg.SupergroupChatID,
		WeekStartDate: baseTime,
	}

	participantsEven := []repositories.User{
		{ID: 1, TgID: 101, Firstname: "Alice", TgUsername: "alice_tg"},
		{ID: 2, TgID: 102, Firstname: "Bob", TgUsername: "bob_tg"},
		{ID: 3, TgID: 103, Firstname: "Charlie", TgUsername: "charlie_tg"},
		{ID: 4, TgID: 104, Firstname: "Diana", TgUsername: ""}, // No username
	}
	participantsOdd := append(participantsEven, repositories.User{ID: 5, TgID: 105, Firstname: "Eve", TgUsername: "eve_tg"})


	tests := []struct {
		name                 string
		user                 *gotgbot.User
		isAdmin              bool
		getLatestPollResult  *models.WeeklyMeetingPoll
		getLatestPollError   error
		getParticipantsResult []repositories.User
		getParticipantsError error
		expectedReply        string // For admin replies
		expectedMessageToGroupContains []string // For messages to supergroup
		expectMessageToGroup bool
	}{
		{
			name:          "Permission Denied",
			user:          regularUser,
			isAdmin:       false,
			expectedReply: "", // No reply for permission denied in current implementation
		},
		{
			name:               "GetLatestPollForChat returns error",
			user:               adminUser,
			isAdmin:            true,
			getLatestPollError: errors.New("db poll error"),
			expectedReply:      "Error fetching poll information.",
		},
		{
			name:                "GetLatestPollForChat returns nil (no poll)",
			user:                adminUser,
			isAdmin:             true,
			getLatestPollResult: nil,
			expectedReply:       fmt.Sprintf("No weekly meeting poll found for chat ID %d.", mockCfg.SupergroupChatID),
		},
		{
			name:                  "GetParticipatingUsers returns error",
			user:                  adminUser,
			isAdmin:               true,
			getLatestPollResult:   mockPoll,
			getParticipantsError:  errors.New("db participant error"),
			expectedReply:         "Error fetching participants.",
		},
		{
			name:                   "Fewer than 2 participants",
			user:                   adminUser,
			isAdmin:                true,
			getLatestPollResult:    mockPoll,
			getParticipantsResult:  []repositories.User{participantsEven[0]},
			expectMessageToGroup:   true,
			expectedMessageToGroupContains: []string{"Not enough participants for pairing", "Need at least 2, got 1"},
		},
		{
			name:                   "Even number of participants",
			user:                   adminUser,
			isAdmin:                true,
			getLatestPollResult:    mockPoll,
			getParticipantsResult:  participantsEven,
			expectMessageToGroup:   true,
			expectedMessageToGroupContains: []string{"Pairs for Random Coffee", "Alice (@alice_tg) - Bob (@bob_tg)", "Charlie (@charlie_tg) - Diana"}, // Order might vary due to shuffle
			expectedReply:          "Pairings announced in the supergroup.",
		},
		{
			name:                   "Odd number of participants",
			user:                   adminUser,
			isAdmin:                true,
			getLatestPollResult:    mockPoll,
			getParticipantsResult:  participantsOdd,
			expectMessageToGroup:   true,
			expectedMessageToGroupContains: []string{"Pairs for Random Coffee", "is looking for a coffee buddy"}, // Check for unpaired message
			expectedReply:          "Pairings announced in the supergroup.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPermissions := new(MockPermissionsService)
			mockSender := new(MockMessageSenderService)
			mockPollRepo := new(MockWeeklyMeetingPollRepository)
			mockParticipantRepo := new(MockWeeklyMeetingParticipantRepository)

			handler := NewPairMeetingsHandler(mockCfg, mockPermissions, mockSender, mockPollRepo, mockParticipantRepo).(*PairMeetingsHandler)

			mockPermissions.On("IsAdmin", tt.user.Id).Return(tt.isAdmin)

			if tt.isAdmin { // Only mock further if admin check passes
				mockPollRepo.On("GetLatestPollForChat", mockCfg.SupergroupChatID).Return(tt.getLatestPollResult, tt.getLatestPollError)
				if tt.getLatestPollError == nil && tt.getLatestPollResult != nil {
					mockParticipantRepo.On("GetParticipatingUsers", mockPoll.ID).Return(tt.getParticipantsResult, tt.getParticipantsError)
				}
			}
			
			// Mock replies to admin
			if tt.expectedReply != "" {
				mockSender.On("Reply", mock.Anything, tt.expectedReply, mock.AnythingOfType("*services.SendReplyOpts")).Return(&gotgbot.Message{}, nil).Once()
			}

			// Mock messages to group
			if tt.expectMessageToGroup {
				mockSender.On("SendMessage", mockCfg.SupergroupChatID, mock.AnythingOfType("string"), mock.AnythingOfType("*services.SendMessageOpts")).Run(func(args mock.Arguments) {
					sentMsg := args.String(1)
					for _, sub := range tt.expectedMessageToGroupContains {
						assert.Contains(t, sentMsg, sub)
					}
					// Specific checks for pairing messages
					if strings.Contains(tt.name, "Even number") || strings.Contains(tt.name, "Odd number") {
						assert.Contains(t, sentMsg, "Pairs for Random Coffee (Week of Mon, Jul 22):")
						assert.Contains(t, sentMsg, "🗓 День, время и формат встречи вы выбираете сами.")
                        // Check that user display names are correct
                        if len(tt.getParticipantsResult) >=2 {
                             // This is tricky due to shuffle. We mainly check one instance of formatting.
                            user1 := tt.getParticipantsResult[0]
                            user2 := tt.getParticipantsResult[1]
                            
                            expectedPairPart1 := user1.Firstname
                            if user1.TgUsername != "" { expectedPairPart1 += " (@" + user1.TgUsername + ")" }
                            
                            expectedPairPart2 := user2.Firstname
                            if user2.TgUsername != "" { expectedPairPart2 += " (@" + user2.TgUsername + ")" }

                            assert.Contains(t, sentMsg, expectedPairPart1)
                            assert.Contains(t, sentMsg, expectedPairPart2)

							// Check Diana (no username) formatting if present
							foundDiana := false
							for _, p := range tt.getParticipantsResult {
								if p.Firstname == "Diana" {
									foundDiana = true
									break
								}
							}
							if foundDiana {
								assert.Contains(t, sentMsg, "Diana")
								assert.NotContains(t, sentMsg, "Diana (@)") 
							}
                        }
						if strings.Contains(tt.name, "Odd number") {
							// Ensure the unpaired user is mentioned correctly. The exact user depends on shuffle.
							// We check that *someone* is unpaired and the message format is there.
							assert.Contains(t, sentMsg, "is looking for a coffee buddy this week!")
						}
					}

				}).Return(&gotgbot.Message{}, nil).Once()
			}


			ctx := &ext.Context{
				EffectiveMessage: &gotgbot.Message{
					MessageId: 100,
					Chat:      &gotgbot.Chat{Id: tt.user.Id}, // Command comes from user's chat
					From:      tt.user,
					Text:      "/" + constants.PairMeetingsCommand,
				},
				EffectiveUser: tt.user,
			}

			err := handler.handleCommand(nil, ctx) // Bot can be nil for this handler if not directly used
			assert.NoError(t, err) // Handler itself should not return errors, it sends messages

			mockPermissions.AssertExpectations(t)
			mockPollRepo.AssertExpectations(t)
			mockParticipantRepo.AssertExpectations(t)
			mockSender.AssertExpectations(t)
		})
	}
}
