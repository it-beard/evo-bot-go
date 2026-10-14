package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type coffeeFixture struct {
	svc    *RandomCoffeeService
	client *testutils.MockBotClient
	mock   sqlmock.Sqlmock
	cfg    *config.Config
}

func newCoffeeFixture(t *testing.T) *coffeeFixture {
	t.Helper()
	bot, client := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	cfg := newTestConfig()
	svc := NewRandomCoffeeService(
		bot,
		cfg,
		NewPollSenderService(bot),
		NewMessageSenderService(bot),
		repositories.NewRandomCoffeePollRepository(db),
		repositories.NewRandomCoffeeParticipantRepository(db),
		repositories.NewProfileRepository(db),
		repositories.NewRandomCoffeePairRepository(db),
		repositories.NewUserRepository(db),
	)
	return &coffeeFixture{svc: svc, client: client, mock: mock, cfg: cfg}
}

func coffeeUser(id int) repositories.User {
	return repositories.User{
		ID:         id,
		TgID:       int64(1000 + id),
		Firstname:  fmt.Sprintf("Name%d", id),
		Lastname:   fmt.Sprintf("Last%d", id),
		TgUsername: fmt.Sprintf("user%d", id),
	}
}

func coffeeUsers(ids ...int) []repositories.User {
	users := make([]repositories.User, 0, len(ids))
	for _, id := range ids {
		users = append(users, coffeeUser(id))
	}
	return users
}

func userIDs(users []repositories.User) []int {
	ids := make([]int, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

// setTelegramUsers makes getChatMember return the given profile data per Telegram user ID
func (f *coffeeFixture) setTelegramUsers(users ...repositories.User) {
	byTgID := make(map[int64]repositories.User, len(users))
	for _, u := range users {
		byTgID[u.TgID] = u
	}
	f.client.On("getChatMember", func(call testutils.APICall) (json.RawMessage, error) {
		u, ok := byTgID[call.Int64("user_id")]
		if !ok {
			return nil, &gotgbot.TelegramError{Method: "getChatMember", Code: 400, Description: "Bad Request: user not found"}
		}
		return json.Marshal(map[string]any{
			"status": "member",
			"user": map[string]any{
				"id": u.TgID, "is_bot": false, "first_name": u.Firstname, "last_name": u.Lastname, "username": u.TgUsername,
			},
		})
	})
}

// currentWeekStart returns the coffee week start the service computes right now
func currentWeekStart() time.Time {
	return nextWeekStartDate(time.Now().UTC())
}

// skipIfWeekChanged skips the assertion phase if a Monday 00:00 UTC boundary was crossed during the test
func skipIfWeekChanged(t *testing.T, weekStart time.Time) {
	t.Helper()
	if !currentWeekStart().Equal(weekStart) {
		t.Skip("coffee week changed while the test was running")
	}
}

// assertValidPairing checks that pairs and the unpaired user form a partition of the participants
func assertValidPairing(t *testing.T, participants []repositories.User, pairs []CoffeePair, unpaired *repositories.User) {
	t.Helper()
	seen := make(map[int]int)
	for _, p := range pairs {
		assert.NotEqual(t, p.User1.ID, p.User2.ID, "a user cannot be paired with themselves")
		seen[p.User1.ID]++
		seen[p.User2.ID]++
	}
	if unpaired != nil {
		seen[unpaired.ID]++
	}
	assert.Len(t, pairs, len(participants)/2)
	assert.Equal(t, len(participants)%2 == 1, unpaired != nil, "only odd groups leave someone unpaired")
	for _, u := range participants {
		assert.Equal(t, 1, seen[u.ID], "user %d must appear exactly once", u.ID)
	}
	assert.Len(t, seen, len(participants))
}

func pairSet(pairs []CoffeePair) map[[2]int]bool {
	set := make(map[[2]int]bool)
	for _, p := range pairs {
		a, b := p.User1.ID, p.User2.ID
		if a > b {
			a, b = b, a
		}
		set[[2]int{a, b}] = true
	}
	return set
}

func dbPairSet(t *testing.T, rec recordingArg) map[[2]int]bool {
	t.Helper()
	set := make(map[[2]int]bool)
	for _, p := range pairsFromRecorder(t, rec) {
		assert.Less(t, p[0], p[1], "pairs are stored with user1_id < user2_id")
		set[[2]int{int(p[0]), int(p[1])}] = true
	}
	return set
}

// --- nextWeekStartDate -----------------------------------------------------------

func TestNextWeekStartDate(t *testing.T) {
	tests := []struct {
		name    string
		now     time.Time
		weekday time.Weekday
		want    time.Time
	}{
		{"monday morning goes to next monday", time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC), time.Monday, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)},
		{"monday midnight goes to next monday", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), time.Monday, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)},
		{"tuesday", time.Date(2026, 10, 13, 15, 30, 0, 0, time.UTC), time.Tuesday, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)},
		{"saturday", time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC), time.Saturday, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)},
		{"sunday late evening", time.Date(2026, 10, 18, 23, 59, 59, 0, time.UTC), time.Sunday, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)},
		{"month boundary", time.Date(2026, 2, 27, 18, 0, 0, 0, time.UTC), time.Friday, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)},
		{"year boundary", time.Date(2025, 12, 31, 8, 0, 0, 0, time.UTC), time.Wednesday, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.weekday, tt.now.Weekday(), "test data sanity check")

			got := nextWeekStartDate(tt.now)

			assert.Equal(t, tt.want, got)
			assert.Equal(t, time.Monday, got.Weekday())
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

// --- IsPollDue / IsPairsDue -------------------------------------------------------

func TestRandomCoffeeService_IsPollDue_WeeklyIntervalNeverQueriesDB(t *testing.T) {
	for _, interval := range []int{0, 1} {
		t.Run(fmt.Sprintf("interval %d", interval), func(t *testing.T) {
			f := newCoffeeFixture(t)
			f.cfg.RandomCoffeeIntervalWeeks = interval

			due, err := f.svc.IsPollDue()

			require.NoError(t, err)
			assert.True(t, due)
		})
	}
}

func TestRandomCoffeeService_IsPollDue_WithoutPollRepo(t *testing.T) {
	f := newCoffeeFixture(t)
	f.cfg.RandomCoffeeIntervalWeeks = 2
	f.svc.pollRepo = nil

	due, err := f.svc.IsPollDue()

	require.NoError(t, err)
	assert.True(t, due)
}

func TestRandomCoffeeService_IsPollDue_MultiWeekInterval(t *testing.T) {
	tests := []struct {
		name           string
		interval       int
		latestWeeksAgo int // latest poll week start = next week start - N weeks; -1 means no poll
		want           bool
	}{
		{name: "no previous poll", interval: 2, latestWeeksAgo: -1, want: true},
		{name: "poll already created for next week", interval: 2, latestWeeksAgo: 0, want: false},
		{name: "poll for current week, interval 2", interval: 2, latestWeeksAgo: 1, want: false},
		{name: "two weeks gap, interval 2", interval: 2, latestWeeksAgo: 2, want: true},
		{name: "two weeks gap, interval 3", interval: 3, latestWeeksAgo: 2, want: false},
		{name: "three weeks gap, interval 3", interval: 3, latestWeeksAgo: 3, want: true},
		{name: "long pause, interval 4", interval: 4, latestWeeksAgo: 10, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCoffeeFixture(t)
			f.cfg.RandomCoffeeIntervalWeeks = tt.interval
			weekStart := currentWeekStart()
			if tt.latestWeeksAgo < 0 {
				expectLatestPoll(f.mock, nil)
			} else {
				expectLatestPoll(f.mock, &repositories.RandomCoffeePoll{
					ID: 3, MessageID: 30, TelegramPollID: "tg-3",
					WeekStartDate: weekStart.AddDate(0, 0, -7*tt.latestWeeksAgo),
				})
			}

			due, err := f.svc.IsPollDue()

			require.NoError(t, err)
			skipIfWeekChanged(t, weekStart)
			assert.Equal(t, tt.want, due)
		})
	}
}

func TestRandomCoffeeService_IsPollDue_DBError(t *testing.T) {
	f := newCoffeeFixture(t)
	f.cfg.RandomCoffeeIntervalWeeks = 2
	f.mock.ExpectQuery(latestPollQuery).WillReturnError(errors.New("db is down"))

	due, err := f.svc.IsPollDue()

	require.Error(t, err)
	assert.False(t, due)
	assert.Contains(t, err.Error(), "db is down")
}

func TestRandomCoffeeService_IsPairsDue_WeeklyIntervalNeverQueriesDB(t *testing.T) {
	f := newCoffeeFixture(t)
	f.cfg.RandomCoffeeIntervalWeeks = 1

	due, err := f.svc.IsPairsDue()

	require.NoError(t, err)
	assert.True(t, due)
}

func TestRandomCoffeeService_IsPairsDue_MultiWeekInterval(t *testing.T) {
	tests := []struct {
		name           string
		interval       int
		latestWeeksAgo int // -1 means no poll
		want           bool
	}{
		{name: "no poll", interval: 2, latestWeeksAgo: -1, want: false},
		{name: "poll for the upcoming week", interval: 2, latestWeeksAgo: 0, want: true},
		{name: "poll for the current week", interval: 2, latestWeeksAgo: 1, want: true},
		{name: "poll from the previous coffee week", interval: 2, latestWeeksAgo: 2, want: false},
		{name: "old poll with interval 3", interval: 3, latestWeeksAgo: 3, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCoffeeFixture(t)
			f.cfg.RandomCoffeeIntervalWeeks = tt.interval
			weekStart := currentWeekStart()
			if tt.latestWeeksAgo < 0 {
				expectLatestPoll(f.mock, nil)
			} else {
				expectLatestPoll(f.mock, &repositories.RandomCoffeePoll{
					ID: 3, MessageID: 30, TelegramPollID: "tg-3",
					WeekStartDate: weekStart.AddDate(0, 0, -7*tt.latestWeeksAgo),
				})
			}

			due, err := f.svc.IsPairsDue()

			require.NoError(t, err)
			skipIfWeekChanged(t, weekStart)
			assert.Equal(t, tt.want, due)
		})
	}
}

func TestRandomCoffeeService_IsPairsDue_DBError(t *testing.T) {
	f := newCoffeeFixture(t)
	f.cfg.RandomCoffeeIntervalWeeks = 3
	f.mock.ExpectQuery(latestPollQuery).WillReturnError(errors.New("db is down"))

	due, err := f.svc.IsPairsDue()

	require.Error(t, err)
	assert.False(t, due)
}

// --- SendPoll ---------------------------------------------------------------------

func (f *coffeeFixture) stubSentPoll(messageID int64, pollID string) {
	f.client.OnJSON("sendPoll", gotgbot.Message{
		MessageId:       messageID,
		Chat:            gotgbot.Chat{Id: testFullChatID, Type: "supergroup"},
		MessageThreadId: testCoffeeTopicID,
		Poll:            &gotgbot.Poll{Id: pollID, Question: "q", Type: "regular"},
	})
}

func TestRandomCoffeeService_SendPoll_Success(t *testing.T) {
	f := newCoffeeFixture(t)
	f.stubSentPoll(4242, "tg-poll-1")
	f.mock.ExpectQuery(createPollQuery).
		WithArgs(int64(4242), mondayArg{}, "tg-poll-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(17))

	err := f.svc.SendPoll(context.Background())

	require.NoError(t, err)

	var methods []string
	for _, c := range f.client.Calls() {
		methods = append(methods, c.Method)
	}
	assert.Equal(t, []string{"sendMessage", "sendPoll", "pinChatMessage"}, methods)

	announcement, _ := f.client.LastCallTo("sendMessage")
	assert.Equal(t, testFullChatID, announcement.Int64("chat_id"))
	assert.Equal(t, int64(testCoffeeTopicID), announcement.Int64("message_thread_id"))
	assert.Equal(t, "HTML", announcement.Params["parse_mode"])
	assert.Contains(t, announcement.Params["text"], "<b>Random Coffee</b>")
	assert.Contains(t, announcement.Params["text"], `<a href="https://telegram.me/c/1234567890/55/56">правила участия</a>`)

	poll, _ := f.client.LastCallTo("sendPoll")
	assert.Equal(t, testFullChatID, poll.Int64("chat_id"))
	assert.Equal(t, int64(testCoffeeTopicID), poll.Int64("message_thread_id"))
	assert.Equal(t, "Будешь участвовать в Random Coffee на следующей неделе? ☕️", poll.Params["question"])
	assert.Equal(t, "false", poll.Params["is_anonymous"], "votes must be visible to match participants")
	assert.Equal(t, "false", poll.Params["allows_multiple_answers"])
	var options []gotgbot.InputPollOption
	require.NoError(t, poll.DecodeJSON("options", &options))
	assert.Equal(t, []gotgbot.InputPollOption{{Text: "Да! 🤗"}, {Text: "Не в этот раз 💁🏽"}}, options)

	pin, _ := f.client.LastCallTo("pinChatMessage")
	assert.Equal(t, testFullChatID, pin.Int64("chat_id"))
	assert.Equal(t, int64(4242), pin.Int64("message_id"))
	assert.Equal(t, "false", pin.Params["disable_notification"], "the poll is pinned with a notification")
}

func TestRandomCoffeeService_SendPoll_TopicNotConfigured(t *testing.T) {
	f := newCoffeeFixture(t)
	f.cfg.RandomCoffeeTopicID = 0

	err := f.svc.SendPoll(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RandomCoffeeTopicID is not configured")
	assert.Empty(t, f.client.Calls())
}

func TestRandomCoffeeService_SendPoll_Failures(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(f *coffeeFixture)
		wantErr     string
		wantMethods []string
	}{
		{
			name:        "announcement fails",
			setup:       func(f *coffeeFixture) { f.client.OnError("sendMessage", "Bad Request: chat not found") },
			wantErr:     "Failed to send regular message",
			wantMethods: []string{"sendMessage"},
		},
		{
			name:        "poll fails",
			setup:       func(f *coffeeFixture) { f.client.OnError("sendPoll", "Bad Request: not enough rights") },
			wantErr:     "not enough rights",
			wantMethods: []string{"sendMessage", "sendPoll"},
		},
		{
			name: "pin fails",
			setup: func(f *coffeeFixture) {
				f.stubSentPoll(4242, "tg-poll-1")
				f.client.OnError("pinChatMessage", "Bad Request: not enough rights")
			},
			wantErr:     "Failed to pin poll",
			wantMethods: []string{"sendMessage", "sendPoll", "pinChatMessage"},
		},
		{
			name: "db insert fails",
			setup: func(f *coffeeFixture) {
				f.stubSentPoll(4242, "tg-poll-1")
				f.mock.ExpectQuery(createPollQuery).WillReturnError(errors.New("unique violation"))
			},
			wantErr:     "unique violation",
			wantMethods: []string{"sendMessage", "sendPoll", "pinChatMessage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCoffeeFixture(t)
			tt.setup(f)

			err := f.svc.SendPoll(context.Background())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			var methods []string
			for _, c := range f.client.Calls() {
				methods = append(methods, c.Method)
			}
			assert.Equal(t, tt.wantMethods, methods)
		})
	}
}

func TestRandomCoffeeService_SendPoll_WithoutPollRepoSkipsDB(t *testing.T) {
	f := newCoffeeFixture(t)
	f.svc.pollRepo = nil

	require.NoError(t, f.svc.SendPoll(context.Background()))

	assert.Len(t, f.client.CallsTo("pinChatMessage"), 1)
}

// --- generateSmartPairs -----------------------------------------------------------

func TestRandomCoffeeService_generateSmartPairs_EvenGroupWithoutHistory(t *testing.T) {
	participants := coffeeUsers(1, 2, 3, 4, 5, 6)
	// The shuffle is seeded with the current time; repeat to exercise many orders
	for i := 0; i < 20; i++ {
		f := newCoffeeFixture(t)
		expectPairHistory(f.mock, userIDs(participants), nil)
		rec := expectCreatePairs(f.mock, 9, 3)

		pairs, unpaired, err := f.svc.generateSmartPairs(participants, 9)

		require.NoError(t, err)
		assertValidPairing(t, participants, pairs, unpaired)
		assert.Equal(t, pairSet(pairs), dbPairSet(t, rec), "every pair is persisted")
	}
}

func TestRandomCoffeeService_generateSmartPairs_OddGroupLeavesOneUnpaired(t *testing.T) {
	participants := coffeeUsers(1, 2, 3, 4, 5)
	for i := 0; i < 20; i++ {
		f := newCoffeeFixture(t)
		expectPairHistory(f.mock, userIDs(participants), nil)
		rec := expectCreatePairs(f.mock, 9, 2)

		pairs, unpaired, err := f.svc.generateSmartPairs(participants, 9)

		require.NoError(t, err)
		require.NotNil(t, unpaired)
		assertValidPairing(t, participants, pairs, unpaired)
		assert.Equal(t, pairSet(pairs), dbPairSet(t, rec))
	}
}

func TestRandomCoffeeService_generateSmartPairs_AvoidsRecentPartnerWhenAlternativeExists(t *testing.T) {
	participants := coffeeUsers(1, 2, 3)
	for i := 0; i < 30; i++ {
		f := newCoffeeFixture(t)
		expectPairHistory(f.mock, userIDs(participants), [][3]int{{1, 2, 5}})
		rec := expectCreatePairs(f.mock, 9, 1)

		pairs, unpaired, err := f.svc.generateSmartPairs(participants, 9)

		require.NoError(t, err)
		assertValidPairing(t, participants, pairs, unpaired)
		assert.False(t, pairSet(pairs)[[2]int{1, 2}], "users 1 and 2 met recently and user 3 is available")
		assert.NotEqual(t, 3, unpaired.ID, "user 3 has never met anyone and must get a partner")
		assert.Equal(t, pairSet(pairs), dbPairSet(t, rec))
	}
}

func TestRandomCoffeeService_generateSmartPairs_NoRepeatsWhenPreviousRoundIsAPerfectMatching(t *testing.T) {
	participants := coffeeUsers(1, 2, 3, 4)
	// Last round paired 1-2 and 3-4; whatever the shuffle order, a fresh matching is always found
	history := [][3]int{{1, 2, 4}, {3, 4, 4}}
	for i := 0; i < 30; i++ {
		f := newCoffeeFixture(t)
		expectPairHistory(f.mock, userIDs(participants), history)
		rec := expectCreatePairs(f.mock, 5, 2)

		pairs, unpaired, err := f.svc.generateSmartPairs(participants, 5)

		require.NoError(t, err)
		assertValidPairing(t, participants, pairs, unpaired)
		got := pairSet(pairs)
		assert.False(t, got[[2]int{1, 2}], "1-2 met last round")
		assert.False(t, got[[2]int{3, 4}], "3-4 met last round")
		assert.Equal(t, got, dbPairSet(t, rec))
	}
}

func TestRandomCoffeeService_generateSmartPairs_PrefersOldestRepeat(t *testing.T) {
	participants := coffeeUsers(1, 2, 3)
	// Everyone met everyone; 1-2 is the most recent pairing and must never be repeated
	history := [][3]int{{1, 2, 10}, {1, 3, 5}, {2, 3, 7}}
	for i := 0; i < 30; i++ {
		f := newCoffeeFixture(t)
		expectPairHistory(f.mock, userIDs(participants), history)
		expectCreatePairs(f.mock, 11, 1)

		pairs, unpaired, err := f.svc.generateSmartPairs(participants, 11)

		require.NoError(t, err)
		assertValidPairing(t, participants, pairs, unpaired)
		assert.False(t, pairSet(pairs)[[2]int{1, 2}], "the most recent pairing is avoided")
	}
}

func TestRandomCoffeeService_generateSmartPairs_RepeatsWhenNoAlternative(t *testing.T) {
	participants := coffeeUsers(1, 2)
	f := newCoffeeFixture(t)
	expectPairHistory(f.mock, userIDs(participants), [][3]int{{2, 1, 8}})
	rec := expectCreatePairs(f.mock, 9, 1)

	pairs, unpaired, err := f.svc.generateSmartPairs(participants, 9)

	require.NoError(t, err)
	assert.Nil(t, unpaired)
	require.Len(t, pairs, 1)
	assert.Equal(t, map[[2]int]bool{{1, 2}: true}, dbPairSet(t, rec))
}

func TestRandomCoffeeService_generateSmartPairs_NotEnoughParticipants(t *testing.T) {
	f := newCoffeeFixture(t)

	pairs, unpaired, err := f.svc.generateSmartPairs(coffeeUsers(1), 9)

	require.Error(t, err)
	assert.Nil(t, pairs)
	assert.Nil(t, unpaired)
}

func TestRandomCoffeeService_generateSmartPairs_HistoryError(t *testing.T) {
	f := newCoffeeFixture(t)
	f.mock.ExpectQuery(pairHistoryQuery).WillReturnError(errors.New("db is down"))

	pairs, unpaired, err := f.svc.generateSmartPairs(coffeeUsers(1, 2), 9)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get pair history")
	assert.Nil(t, pairs)
	assert.Nil(t, unpaired)
}

func TestRandomCoffeeService_generateSmartPairs_SaveErrorIsTolerated(t *testing.T) {
	participants := coffeeUsers(1, 2, 3, 4)
	f := newCoffeeFixture(t)
	expectPairHistory(f.mock, userIDs(participants), nil)
	f.mock.ExpectExec(createPairQuery).WillReturnError(errors.New("insert failed"))
	f.mock.ExpectExec(createPairQuery).WillReturnResult(sqlmock.NewResult(1, 1))

	pairs, unpaired, err := f.svc.generateSmartPairs(participants, 9)

	require.NoError(t, err)
	assertValidPairing(t, participants, pairs, unpaired)
}

func TestRandomCoffeeService_generateSmartPairs_DoesNotMutateInput(t *testing.T) {
	participants := coffeeUsers(1, 2, 3, 4, 5, 6, 7, 8)
	original := append([]repositories.User(nil), participants...)
	f := newCoffeeFixture(t)
	expectPairHistory(f.mock, userIDs(participants), nil)
	expectCreatePairs(f.mock, 9, 4)

	_, _, err := f.svc.generateSmartPairs(participants, 9)

	require.NoError(t, err)
	assert.Equal(t, original, participants)
}

// --- createPairsFromShuffled ---------------------------------------------------------

func TestRandomCoffeeService_createPairsFromShuffled(t *testing.T) {
	t.Run("odd group pairs neighbours and leaves the last one", func(t *testing.T) {
		f := newCoffeeFixture(t)
		participants := coffeeUsers(3, 1, 2, 5, 4)
		f.mock.ExpectExec(createPairQuery).WithArgs(7, 1, 3).WillReturnResult(sqlmock.NewResult(1, 1))
		f.mock.ExpectExec(createPairQuery).WithArgs(7, 2, 5).WillReturnResult(sqlmock.NewResult(2, 1))

		pairs, unpaired := f.svc.createPairsFromShuffled(participants, 7)

		require.Len(t, pairs, 2)
		assert.Equal(t, 3, pairs[0].User1.ID)
		assert.Equal(t, 1, pairs[0].User2.ID)
		assert.Equal(t, 2, pairs[1].User1.ID)
		assert.Equal(t, 5, pairs[1].User2.ID)
		require.NotNil(t, unpaired)
		assert.Equal(t, 4, unpaired.ID)
	})

	t.Run("even group", func(t *testing.T) {
		f := newCoffeeFixture(t)
		participants := coffeeUsers(1, 2, 3, 4)
		f.mock.ExpectExec(createPairQuery).WithArgs(7, 1, 2).WillReturnResult(sqlmock.NewResult(1, 1))
		f.mock.ExpectExec(createPairQuery).WithArgs(7, 3, 4).WillReturnResult(sqlmock.NewResult(2, 1))

		pairs, unpaired := f.svc.createPairsFromShuffled(participants, 7)

		assert.Len(t, pairs, 2)
		assert.Nil(t, unpaired)
	})

	t.Run("save errors are tolerated", func(t *testing.T) {
		f := newCoffeeFixture(t)
		f.mock.ExpectExec(createPairQuery).WillReturnError(errors.New("insert failed"))

		pairs, unpaired := f.svc.createPairsFromShuffled(coffeeUsers(1, 2), 7)

		assert.Len(t, pairs, 1)
		assert.Nil(t, unpaired)
	})

	t.Run("without pair repository", func(t *testing.T) {
		f := newCoffeeFixture(t)
		f.svc.pairRepo = nil

		pairs, unpaired := f.svc.createPairsFromShuffled(coffeeUsers(1, 2, 3), 7)

		assert.Len(t, pairs, 1)
		require.NotNil(t, unpaired)
		assert.Equal(t, 3, unpaired.ID)
	})
}

// --- formatUserDisplay ---------------------------------------------------------------

func TestRandomCoffeeService_formatUserDisplay(t *testing.T) {
	t.Run("username with published profile", func(t *testing.T) {
		f := newCoffeeFixture(t)
		user := coffeeUser(1)
		expectProfile(f.mock, 1, 9001)

		got := f.svc.formatUserDisplay(&user)

		assert.Equal(t, `@user1 <i>(<a href="https://telegram.me/c/1234567890/77/9001">профиль</a>)</i>`, got)
	})

	t.Run("first name without username or profile", func(t *testing.T) {
		f := newCoffeeFixture(t)
		user := coffeeUser(2)
		user.TgUsername = ""
		expectProfile(f.mock, 2, 0)

		assert.Equal(t, "Name2", f.svc.formatUserDisplay(&user))
	})

	t.Run("published message id zero is ignored", func(t *testing.T) {
		f := newCoffeeFixture(t)
		user := coffeeUser(3)
		now := time.Now()
		f.mock.ExpectQuery(profileByUserID).WithArgs(3).
			WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(30, 3, "bio", int64(0), now, now))

		assert.Equal(t, "@user3", f.svc.formatUserDisplay(&user))
	})

	t.Run("profile lookup error", func(t *testing.T) {
		f := newCoffeeFixture(t)
		user := coffeeUser(4)
		f.mock.ExpectQuery(profileByUserID).WithArgs(4).WillReturnError(errors.New("db is down"))

		assert.Equal(t, "@user4", f.svc.formatUserDisplay(&user))
	})
}

// --- GenerateAndSendPairs -----------------------------------------------------------

var (
	pairLineRe   = regexp.MustCompile(`(?m)^➪ (.+)$`)
	usernameRe   = regexp.MustCompile(`@user(\d+)|@fresh(\d+)`)
	unpairedRe   = regexp.MustCompile(`(?m)^😔 (.+) без пары`)
	testPollWeek = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
)

func userIDsInText(t *testing.T, text string) []int {
	t.Helper()
	var ids []int
	for _, m := range usernameRe.FindAllStringSubmatch(text, -1) {
		raw := m[1]
		if raw == "" {
			raw = m[2]
		}
		var id int
		_, err := fmt.Sscanf(raw, "%d", &id)
		require.NoError(t, err)
		ids = append(ids, id)
	}
	return ids
}

// parseAnnouncement extracts pairs (sorted user IDs) and the unpaired user ID (0 if none) from the message
func parseAnnouncement(t *testing.T, text string) (map[[2]int]bool, int) {
	t.Helper()
	pairs := make(map[[2]int]bool)
	for _, m := range pairLineRe.FindAllStringSubmatch(text, -1) {
		ids := userIDsInText(t, m[1])
		require.Len(t, ids, 2, "pair line %q", m[1])
		sort.Ints(ids)
		pairs[[2]int{ids[0], ids[1]}] = true
	}
	unpaired := 0
	if m := unpairedRe.FindStringSubmatch(text); m != nil {
		ids := userIDsInText(t, m[1])
		require.Len(t, ids, 1)
		unpaired = ids[0]
	}
	return pairs, unpaired
}

func (f *coffeeFixture) expectPollAndParticipants(participants []repositories.User) {
	expectLatestPoll(f.mock, &repositories.RandomCoffeePoll{ID: 7, MessageID: 333, TelegramPollID: "tg-7", WeekStartDate: testPollWeek})
	expectParticipants(f.mock, 7, participants)
}

func TestRandomCoffeeService_GenerateAndSendPairs_EvenGroup(t *testing.T) {
	f := newCoffeeFixture(t)
	f.mock.MatchExpectationsInOrder(false) // pairing order is random
	participants := coffeeUsers(1, 2, 3, 4)
	f.client.OnJSON("stopPoll", gotgbot.Poll{Id: "tg-7", IsClosed: true, Type: "regular"})
	f.setTelegramUsers(participants...)
	f.expectPollAndParticipants(participants)
	for _, u := range participants {
		expectUserByTgID(f.mock, u)
	}
	expectPairHistory(f.mock, userIDs(participants), nil)
	rec := expectCreatePairs(f.mock, 7, 2)
	expectProfile(f.mock, 1, 9001)
	for _, id := range []int{2, 3, 4} {
		expectProfile(f.mock, id, 0)
	}

	require.NoError(t, f.svc.GenerateAndSendPairs())

	stop, ok := f.client.LastCallTo("stopPoll")
	require.True(t, ok, "the poll is closed before pairing")
	assert.Equal(t, testFullChatID, stop.Int64("chat_id"))
	assert.Equal(t, int64(333), stop.Int64("message_id"))
	for _, member := range f.client.CallsTo("getChatMember") {
		assert.Equal(t, testFullChatID, member.Int64("chat_id"))
	}
	assert.Len(t, f.client.CallsTo("getChatMember"), 4)

	sends := f.client.CallsTo("sendMessage")
	require.Len(t, sends, 1)
	msg := sends[0]
	assert.Equal(t, testFullChatID, msg.Int64("chat_id"))
	assert.Equal(t, int64(testCoffeeTopicID), msg.Int64("message_thread_id"))
	assert.Equal(t, "HTML", msg.Params["parse_mode"])
	text := msg.Params["text"]
	assert.True(t, strings.HasPrefix(text, "☕️ Пары для рандом кофе ➪ <b><i>неделя Mon, Oct 19</i></b>:\n\n"), text)
	assert.True(t, strings.HasSuffix(text, "Просто напиши своей паре в личку, когда и в каком формате тебе удобно встретиться."), text)
	assert.NotContains(t, text, "без пары")
	assert.Contains(t, text, `@user1 <i>(<a href="https://telegram.me/c/1234567890/77/9001">профиль</a>)</i>`)

	announced, unpaired := parseAnnouncement(t, text)
	assert.Zero(t, unpaired)
	assert.Len(t, announced, 2)
	assert.Equal(t, dbPairSet(t, rec), announced, "announced pairs match the persisted ones")
	assert.ElementsMatch(t, []int{1, 2, 3, 4}, userIDsInText(t, text))

	pin, ok := f.client.LastCallTo("pinChatMessage")
	require.True(t, ok)
	assert.Equal(t, testFullChatID, pin.Int64("chat_id"))
	assert.Equal(t, "true", pin.Params["disable_notification"], "pairs are pinned silently")
	assert.NotZero(t, pin.Int64("message_id"))
}

func TestRandomCoffeeService_GenerateAndSendPairs_OddGroup(t *testing.T) {
	f := newCoffeeFixture(t)
	f.mock.MatchExpectationsInOrder(false)
	participants := coffeeUsers(1, 2, 3)
	f.client.OnJSON("stopPoll", gotgbot.Poll{Id: "tg-7", IsClosed: true, Type: "regular"})
	f.setTelegramUsers(participants...)
	f.expectPollAndParticipants(participants)
	for _, u := range participants {
		expectUserByTgID(f.mock, u)
		expectProfile(f.mock, u.ID, 0)
	}
	expectPairHistory(f.mock, userIDs(participants), [][3]int{{1, 2, 6}})
	rec := expectCreatePairs(f.mock, 7, 1)

	require.NoError(t, f.svc.GenerateAndSendPairs())

	texts := f.client.SentTexts()
	require.Len(t, texts, 1)
	announced, unpaired := parseAnnouncement(t, texts[0])
	assert.Len(t, announced, 1)
	assert.NotZero(t, unpaired, "the odd participant is announced as unpaired")
	assert.False(t, announced[[2]int{1, 2}], "recent partners are not paired again")
	assert.Equal(t, dbPairSet(t, rec), announced)
	assert.ElementsMatch(t, []int{1, 2, 3}, userIDsInText(t, texts[0]))
}

func TestRandomCoffeeService_GenerateAndSendPairs_SyncsTelegramProfiles(t *testing.T) {
	f := newCoffeeFixture(t)
	f.mock.MatchExpectationsInOrder(false)
	participants := coffeeUsers(1, 2, 3, 4)

	fresh3 := coffeeUser(3)
	fresh3.TgUsername, fresh3.Firstname, fresh3.Lastname = "fresh3", "NewName3", "NewLast3"
	changed4 := coffeeUser(4)
	changed4.TgUsername, changed4.Firstname, changed4.Lastname = "fresh4", "NewName4", "NewLast4"
	// user 1: DB lookup fails; user 2: Telegram lookup fails (not registered in the fake)
	f.setTelegramUsers(coffeeUser(1), fresh3, changed4)
	f.client.OnJSON("stopPoll", gotgbot.Poll{Id: "tg-7", IsClosed: true, Type: "regular"})

	f.expectPollAndParticipants(participants)
	f.mock.ExpectQuery(userByTgIDQuery).WithArgs(int64(1001)).WillReturnError(errors.New("db is down"))
	expectUserByTgID(f.mock, coffeeUser(2))
	expectUserByTgID(f.mock, coffeeUser(3))
	expectUserByTgID(f.mock, coffeeUser(4))
	// user 3: all three updates succeed
	f.mock.ExpectExec(updateUsername).WithArgs("fresh3", 3).WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectExec(updateFirstname).WithArgs("NewName3", 3).WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectExec(updateLastname).WithArgs("NewLast3", 3).WillReturnResult(sqlmock.NewResult(0, 1))
	// user 4: all three updates fail, so the old data is kept for the announcement
	f.mock.ExpectExec(updateUsername).WithArgs("fresh4", 4).WillReturnError(errors.New("update failed"))
	f.mock.ExpectExec(updateFirstname).WithArgs("NewName4", 4).WillReturnResult(sqlmock.NewResult(0, 0))
	f.mock.ExpectExec(updateLastname).WithArgs("NewLast4", 4).WillReturnError(errors.New("update failed"))
	expectPairHistory(f.mock, userIDs(participants), nil)
	expectCreatePairs(f.mock, 7, 2)
	for _, id := range []int{1, 2, 3, 4} {
		expectProfile(f.mock, id, 0)
	}

	require.NoError(t, f.svc.GenerateAndSendPairs())

	assert.Len(t, f.client.CallsTo("getChatMember"), 3, "no Telegram lookup for a user missing in the DB")
	texts := f.client.SentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "@fresh3", "successfully synced username is announced")
	assert.NotContains(t, texts[0], "@user3")
	assert.Contains(t, texts[0], "@user4", "failed sync keeps the old username")
	assert.NotContains(t, texts[0], "@fresh4")
	assert.Contains(t, texts[0], "@user1")
	assert.Contains(t, texts[0], "@user2")
}

func TestRandomCoffeeService_GenerateAndSendPairs_FallsBackWhenHistoryFails(t *testing.T) {
	f := newCoffeeFixture(t)
	f.mock.MatchExpectationsInOrder(false)
	participants := coffeeUsers(1, 2, 3, 4, 5)
	f.setTelegramUsers(participants...)
	// stopPoll is left with the fake's default response, which cannot be decoded as a Poll: pairing must continue
	f.expectPollAndParticipants(participants)
	for _, u := range participants {
		expectUserByTgID(f.mock, u)
		expectProfile(f.mock, u.ID, 0)
	}
	f.mock.ExpectQuery(pairHistoryQuery).WillReturnError(errors.New("db is down"))
	rec := expectCreatePairs(f.mock, 7, 2)

	require.NoError(t, f.svc.GenerateAndSendPairs())

	assert.Len(t, f.client.CallsTo("stopPoll"), 1)
	texts := f.client.SentTexts()
	require.Len(t, texts, 1)
	announced, unpaired := parseAnnouncement(t, texts[0])
	assert.Len(t, announced, 2)
	assert.NotZero(t, unpaired)
	assert.Equal(t, dbPairSet(t, rec), announced)
	assert.ElementsMatch(t, []int{1, 2, 3, 4, 5}, userIDsInText(t, texts[0]))
}

func TestRandomCoffeeService_GenerateAndSendPairs_PinFailureIsTolerated(t *testing.T) {
	f := newCoffeeFixture(t)
	f.mock.MatchExpectationsInOrder(false)
	participants := coffeeUsers(1, 2)
	f.setTelegramUsers(participants...)
	f.client.OnError("pinChatMessage", "Bad Request: not enough rights")
	f.expectPollAndParticipants(participants)
	for _, u := range participants {
		expectUserByTgID(f.mock, u)
		expectProfile(f.mock, u.ID, 0)
	}
	expectPairHistory(f.mock, userIDs(participants), nil)
	expectCreatePairs(f.mock, 7, 1)

	require.NoError(t, f.svc.GenerateAndSendPairs())

	assert.Len(t, f.client.CallsTo("sendMessage"), 1)
	assert.Len(t, f.client.CallsTo("pinChatMessage"), 1)
}

func TestRandomCoffeeService_GenerateAndSendPairs_SendFailure(t *testing.T) {
	f := newCoffeeFixture(t)
	f.mock.MatchExpectationsInOrder(false)
	participants := coffeeUsers(1, 2)
	f.setTelegramUsers(participants...)
	f.client.OnError("sendMessage", "Bad Request: chat not found")
	f.expectPollAndParticipants(participants)
	for _, u := range participants {
		expectUserByTgID(f.mock, u)
		expectProfile(f.mock, u.ID, 0)
	}
	expectPairHistory(f.mock, userIDs(participants), nil)
	expectCreatePairs(f.mock, 7, 1)

	err := f.svc.GenerateAndSendPairs()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error sending pairing message")
	assert.Empty(t, f.client.CallsTo("pinChatMessage"))
}

func TestRandomCoffeeService_GenerateAndSendPairs_EarlyFailures(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(f *coffeeFixture)
		wantErr   string
		wantStops int
	}{
		{
			name:    "latest poll query fails",
			setup:   func(f *coffeeFixture) { f.mock.ExpectQuery(latestPollQuery).WillReturnError(errors.New("db is down")) },
			wantErr: "error getting latest poll",
		},
		{
			name:    "no poll",
			setup:   func(f *coffeeFixture) { expectLatestPoll(f.mock, nil) },
			wantErr: "опрос для рандом кофе не найден",
		},
		{
			name: "participants query fails",
			setup: func(f *coffeeFixture) {
				expectLatestPoll(f.mock, &repositories.RandomCoffeePoll{ID: 7, MessageID: 333, WeekStartDate: testPollWeek})
				f.mock.ExpectQuery(participantsQuery).WithArgs(int64(7)).WillReturnError(errors.New("db is down"))
			},
			wantErr:   "error getting participants for poll ID 7",
			wantStops: 1,
		},
		{
			name: "single participant",
			setup: func(f *coffeeFixture) {
				f.expectPollAndParticipants(coffeeUsers(1))
			},
			wantErr:   "зарегистрировалось 1",
			wantStops: 1,
		},
		{
			name: "no participants",
			setup: func(f *coffeeFixture) {
				f.expectPollAndParticipants(nil)
			},
			wantErr:   "зарегистрировалось 0",
			wantStops: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCoffeeFixture(t)
			f.client.OnJSON("stopPoll", gotgbot.Poll{Id: "tg-7", IsClosed: true, Type: "regular"})
			tt.setup(f)

			err := f.svc.GenerateAndSendPairs()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Len(t, f.client.CallsTo("stopPoll"), tt.wantStops)
			assert.Empty(t, f.client.CallsTo("sendMessage"))
		})
	}
}
