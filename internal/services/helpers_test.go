package services

import (
	"database/sql/driver"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

const (
	testSuperGroupID   int64 = 1234567890
	testCoffeeTopicID        = 55
	testIntroTopicID         = 77
	testSummaryTopicID       = 88
)

// testFullChatID is the full supergroup chat ID the services send to
var testFullChatID = utils.ChatIdToFullChatId(testSuperGroupID)

func newTestConfig() *config.Config {
	return &config.Config{
		SuperGroupChatID:          testSuperGroupID,
		RandomCoffeeTopicID:       testCoffeeTopicID,
		IntroTopicID:              testIntroTopicID,
		SummaryTopicID:            testSummaryTopicID,
		RandomCoffeeIntervalWeeks: 1,
	}
}

// waitFor polls cond until it returns true or the timeout expires
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// chdirTemp switches the working directory to a fresh temp dir for the duration of the test.
// Tests using it must not run in parallel.
func chdirTemp(t *testing.T) string {
	t.Helper()
	prev, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return dir
}

// recordingArg is a sqlmock argument matcher that accepts any value and records it
type recordingArg struct {
	mu     *sync.Mutex
	values *[]driver.Value
}

func newRecordingArg() recordingArg {
	return recordingArg{mu: &sync.Mutex{}, values: &[]driver.Value{}}
}

func (r recordingArg) Match(v driver.Value) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	*r.values = append(*r.values, v)
	return true
}

func (r recordingArg) Int64s() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]int64, 0, len(*r.values))
	for _, v := range *r.values {
		result = append(result, v.(int64))
	}
	return result
}

// mondayArg matches the week start date written by the coffee service: a Monday 00:00 UTC within the next 7 days
type mondayArg struct{}

func (mondayArg) Match(v driver.Value) bool {
	ts, ok := v.(time.Time)
	if !ok {
		return false
	}
	now := time.Now().UTC()
	return ts.Location() == time.UTC &&
		ts.Weekday() == time.Monday &&
		ts.Hour() == 0 && ts.Minute() == 0 && ts.Second() == 0 && ts.Nanosecond() == 0 &&
		ts.After(now.Add(-time.Minute)) && !ts.After(now.Add(7*24*time.Hour))
}

// --- SQL expectation helpers -------------------------------------------------

var (
	latestPollQuery   = regexp.QuoteMeta("FROM random_coffee_polls") + `\s+` + regexp.QuoteMeta("ORDER BY week_start_date DESC, id DESC")
	createPollQuery   = regexp.QuoteMeta("INSERT INTO random_coffee_polls (message_id, week_start_date, telegram_poll_id, created_at)")
	participantsQuery = regexp.QuoteMeta("FROM users u") + `\s+` + regexp.QuoteMeta("JOIN random_coffee_participants rpc")
	userByTgIDQuery   = regexp.QuoteMeta("FROM users") + `\s+` + regexp.QuoteMeta("WHERE tg_id = $1")
	pairHistoryQuery  = regexp.QuoteMeta("SELECT p.user1_id, p.user2_id, poll.id as poll_id")
	createPairQuery   = regexp.QuoteMeta("INSERT INTO random_coffee_pairs (poll_id, user1_id, user2_id)")
	profileByUserID   = regexp.QuoteMeta("FROM profiles") + `\s+` + regexp.QuoteMeta("WHERE user_id = $1")
	updateUsername    = regexp.QuoteMeta("UPDATE users SET tg_username = $1")
	updateFirstname   = regexp.QuoteMeta("UPDATE users SET firstname = $1")
	updateLastname    = regexp.QuoteMeta("UPDATE users SET lastname = $1")
	groupTopicQuery   = regexp.QuoteMeta("FROM group_topics") + `\s+` + regexp.QuoteMeta("WHERE topic_id = $1")
	lastDayMessages   = regexp.QuoteMeta("FROM group_messages") + `\s+` + regexp.QuoteMeta("WHERE group_topic_id = $1 AND created_at >= NOW() - INTERVAL '24 hours'")
	templateQuery     = regexp.QuoteMeta("SELECT template_text FROM prompting_templates WHERE template_key = $1")
	templateInsert    = regexp.QuoteMeta("INSERT INTO prompting_templates (template_key, template_text)")
)

var (
	pollColumns        = []string{"id", "message_id", "week_start_date", "telegram_poll_id", "created_at"}
	participantColumns = []string{"id", "tg_id", "firstname", "lastname", "tg_username"}
	userColumns        = []string{"id", "tg_id", "firstname", "lastname", "tg_username", "score", "has_coffee_ban", "is_club_member", "created_at", "updated_at"}
	profileColumns     = []string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}
	pairHistoryColumns = []string{"user1_id", "user2_id", "poll_id"}
	groupTopicColumns  = []string{"id", "topic_id", "name", "created_at", "updated_at"}
	groupMsgColumns    = []string{"id", "message_id", "message_text", "reply_to_message_id", "user_tg_id", "group_topic_id", "created_at", "updated_at"}
)

func expectLatestPoll(mock sqlmock.Sqlmock, poll *repositories.RandomCoffeePoll) {
	rows := sqlmock.NewRows(pollColumns)
	if poll != nil {
		rows.AddRow(poll.ID, poll.MessageID, poll.WeekStartDate, poll.TelegramPollID, time.Now())
	}
	mock.ExpectQuery(latestPollQuery).WillReturnRows(rows)
}

func expectParticipants(mock sqlmock.Sqlmock, pollID int64, users []repositories.User) {
	rows := sqlmock.NewRows(participantColumns)
	for _, u := range users {
		rows.AddRow(u.ID, u.TgID, u.Firstname, u.Lastname, u.TgUsername)
	}
	mock.ExpectQuery(participantsQuery).WithArgs(pollID).WillReturnRows(rows)
}

func expectUserByTgID(mock sqlmock.Sqlmock, u repositories.User) {
	now := time.Now()
	mock.ExpectQuery(userByTgIDQuery).WithArgs(u.TgID).WillReturnRows(
		sqlmock.NewRows(userColumns).AddRow(u.ID, u.TgID, u.Firstname, u.Lastname, u.TgUsername, 0, false, true, now, now),
	)
}

// expectProfile expects a profile lookup for userID; publishedMessageID 0 means "not published"
func expectProfile(mock sqlmock.Sqlmock, userID int, publishedMessageID int64) {
	var published any
	if publishedMessageID != 0 {
		published = publishedMessageID
	}
	now := time.Now()
	mock.ExpectQuery(profileByUserID).WithArgs(userID).WillReturnRows(
		sqlmock.NewRows(profileColumns).AddRow(userID*10, userID, "bio", published, now, now),
	)
}

func expectPairHistory(mock sqlmock.Sqlmock, userIDs []int, history [][3]int) {
	args := []driver.Value{4}
	for _, id := range userIDs {
		args = append(args, id)
	}
	rows := sqlmock.NewRows(pairHistoryColumns)
	for _, h := range history {
		rows.AddRow(h[0], h[1], h[2])
	}
	mock.ExpectQuery(pairHistoryQuery).WithArgs(args...).WillReturnRows(rows)
}

// expectCreatePairs expects n pair inserts for pollID and returns a recorder of (user1_id, user2_id) values
func expectCreatePairs(mock sqlmock.Sqlmock, pollID int, n int) recordingArg {
	rec := newRecordingArg()
	for i := 0; i < n; i++ {
		mock.ExpectExec(createPairQuery).WithArgs(pollID, rec, rec).WillReturnResult(sqlmock.NewResult(int64(i+1), 1))
	}
	return rec
}

// pairsFromRecorder groups recorded insert args into [user1_id, user2_id] pairs
func pairsFromRecorder(t *testing.T, rec recordingArg) [][2]int64 {
	t.Helper()
	values := rec.Int64s()
	require.Equal(t, 0, len(values)%2, "recorded values must come in pairs")
	pairs := make([][2]int64, 0, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		pairs = append(pairs, [2]int64{values[i], values[i+1]})
	}
	return pairs
}
