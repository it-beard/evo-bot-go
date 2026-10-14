// Package testutils provides shared helpers for unit tests: a fake Telegram Bot API client,
// update builders, a sqlmock-backed database and a fake OpenAI server.
// It must only be imported from _test.go files.
package testutils

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const (
	TestBotID       int64 = 7777
	TestBotUsername       = "test_evo_bot"
)

// APICall is a single request the bot sent to the Telegram Bot API
type APICall struct {
	Method string
	Params map[string]string
}

// Int64 returns an int64 param or 0 if it is missing
func (c APICall) Int64(key string) int64 {
	v, _ := strconv.ParseInt(c.Params[key], 10, 64)
	return v
}

// DecodeJSON unmarshals a JSON-encoded param (e.g. reply_markup) into v
func (c APICall) DecodeJSON(key string, v any) error {
	raw, ok := c.Params[key]
	if !ok {
		return fmt.Errorf("param %q is missing in %s call", key, c.Method)
	}
	return json.Unmarshal([]byte(raw), v)
}

// ResponseFunc builds a raw Bot API response for a call
type ResponseFunc func(call APICall) (json.RawMessage, error)

// MockBotClient implements gotgbot.BotClient without any network access.
// It records every call and answers with sensible defaults that can be overridden per method.
type MockBotClient struct {
	mu                  sync.Mutex
	calls               []APICall
	responses           map[string]ResponseFunc
	memberStatuses      map[int64]string
	defaultMemberStatus string
	nextMessageID       int64
}

// NewMockBotClient creates a client where every user is a regular club member
func NewMockBotClient() *MockBotClient {
	return &MockBotClient{
		responses:           make(map[string]ResponseFunc),
		memberStatuses:      make(map[int64]string),
		defaultMemberStatus: "member",
		nextMessageID:       1000,
	}
}

// NewTestBot creates a bot backed by a MockBotClient
func NewTestBot() (*gotgbot.Bot, *MockBotClient) {
	client := NewMockBotClient()
	bot, err := gotgbot.NewBot("7777:test-token", &gotgbot.BotOpts{
		BotClient:         client,
		DisableTokenCheck: true,
	})
	if err != nil {
		panic(err)
	}
	bot.User = gotgbot.User{Id: TestBotID, IsBot: true, FirstName: "Test Bot", Username: TestBotUsername}
	return bot, client
}

// On overrides the response for a Bot API method
func (m *MockBotClient) On(method string, fn ResponseFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[method] = fn
}

// OnJSON makes a Bot API method return v as its result
func (m *MockBotClient) OnJSON(method string, v any) {
	m.On(method, func(APICall) (json.RawMessage, error) {
		return json.Marshal(v)
	})
}

// OnError makes a Bot API method fail with a Telegram error carrying the given description
func (m *MockBotClient) OnError(method string, description string) {
	m.On(method, func(APICall) (json.RawMessage, error) {
		return nil, &gotgbot.TelegramError{Method: method, Code: 400, Description: description}
	})
}

// SetMemberStatus sets the getChatMember status for a user ("member", "administrator", "creator", "left", "kicked", "restricted")
func (m *MockBotClient) SetMemberStatus(userID int64, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberStatuses[userID] = status
}

// SetDefaultMemberStatus sets the getChatMember status for users without an explicit status
func (m *MockBotClient) SetDefaultMemberStatus(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultMemberStatus = status
}

// Calls returns all recorded calls
func (m *MockBotClient) Calls() []APICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]APICall(nil), m.calls...)
}

// CallsTo returns recorded calls of a given method
func (m *MockBotClient) CallsTo(method string) []APICall {
	var result []APICall
	for _, c := range m.Calls() {
		if c.Method == method {
			result = append(result, c)
		}
	}
	return result
}

// LastCallTo returns the most recent call of a given method
func (m *MockBotClient) LastCallTo(method string) (APICall, bool) {
	calls := m.CallsTo(method)
	if len(calls) == 0 {
		return APICall{}, false
	}
	return calls[len(calls)-1], true
}

// SentTexts returns the "text" param of every sendMessage call
func (m *MockBotClient) SentTexts() []string {
	var texts []string
	for _, c := range m.CallsTo("sendMessage") {
		texts = append(texts, c.Params["text"])
	}
	return texts
}

// Reset forgets all recorded calls (overrides are kept)
func (m *MockBotClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
}

// RequestWithContext implements gotgbot.BotClient
func (m *MockBotClient) RequestWithContext(
	_ context.Context,
	_ string,
	method string,
	params map[string]string,
	_ map[string]gotgbot.FileReader,
	_ *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	call := APICall{Method: method, Params: maps.Clone(params)}

	m.mu.Lock()
	m.calls = append(m.calls, call)
	fn, ok := m.responses[method]
	m.mu.Unlock()

	if ok {
		return fn(call)
	}
	return m.defaultResponse(call)
}

// GetAPIURL implements gotgbot.BotClient
func (m *MockBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL
}

// FileURL implements gotgbot.BotClient
func (m *MockBotClient) FileURL(token string, tgFilePath string, _ *gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL + "/file/bot" + token + "/" + tgFilePath
}

func (m *MockBotClient) newMessageID() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextMessageID++
	return m.nextMessageID
}

func (m *MockBotClient) memberStatus(userID int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if status, ok := m.memberStatuses[userID]; ok {
		return status
	}
	return m.defaultMemberStatus
}

func (m *MockBotClient) defaultResponse(call APICall) (json.RawMessage, error) {
	switch call.Method {
	case "sendMessage", "sendPhoto", "sendAnimation", "sendVideo", "sendDocument",
		"forwardMessage", "editMessageText", "editMessageCaption", "editMessageReplyMarkup":
		return json.Marshal(m.messageFromCall(call))
	case "sendPoll":
		msg := m.messageFromCall(call)
		msg.Poll = &gotgbot.Poll{
			Id:       fmt.Sprintf("poll-%d", msg.MessageId),
			Question: call.Params["question"],
			Type:     "regular",
		}
		return json.Marshal(msg)
	case "copyMessage":
		return json.Marshal(gotgbot.MessageId{MessageId: m.newMessageID()})
	case "getChatMember":
		userID := call.Int64("user_id")
		return json.Marshal(map[string]any{
			"status": m.memberStatus(userID),
			"user":   map[string]any{"id": userID, "is_bot": false, "first_name": "User"},
		})
	case "getChat":
		return json.Marshal(map[string]any{
			"id":                 call.Int64("chat_id"),
			"type":               "supergroup",
			"accent_color_id":    0,
			"max_reaction_count": 0,
		})
	case "getMe":
		return json.Marshal(gotgbot.User{Id: TestBotID, IsBot: true, FirstName: "Test Bot", Username: TestBotUsername})
	default:
		return json.RawMessage("true"), nil
	}
}

func (m *MockBotClient) messageFromCall(call APICall) gotgbot.Message {
	chatID := call.Int64("chat_id")
	msg := gotgbot.Message{
		MessageId:       m.newMessageID(),
		Chat:            gotgbot.Chat{Id: chatID, Type: "supergroup"},
		Date:            1700000000,
		Text:            call.Params["text"],
		Caption:         call.Params["caption"],
		MessageThreadId: call.Int64("message_thread_id"),
	}
	if id := call.Int64("message_id"); id != 0 && call.Method != "forwardMessage" {
		msg.MessageId = id
	}
	if chatID > 0 {
		msg.Chat.Type = "private"
	}
	return msg
}
