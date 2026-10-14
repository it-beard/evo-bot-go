package testutils

import (
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const (
	PrivateChatType    = "private"
	SuperGroupChatType = "supergroup"
)

var (
	updateIDCounter  int64
	messageIDCounter int64 = 100
)

// NewUser builds a Telegram user
func NewUser(userID int64) *gotgbot.User {
	return &gotgbot.User{Id: userID, FirstName: "First", LastName: "Last", Username: "user" + itoa(userID)}
}

// NewPrivateMessage builds a text message from a user in their private chat with the bot
func NewPrivateMessage(userID int64, text string) *gotgbot.Message {
	return NewMessage(userID, userID, PrivateChatType, text)
}

// NewGroupMessage builds a text message in a supergroup chat, optionally inside a topic (threadID != 0)
func NewGroupMessage(userID int64, chatID int64, threadID int64, text string) *gotgbot.Message {
	msg := NewMessage(userID, chatID, SuperGroupChatType, text)
	if threadID != 0 {
		msg.MessageThreadId = threadID
		msg.IsTopicMessage = true
	}
	return msg
}

// NewMessage builds a text message; a leading "/command" gets a bot_command entity like Telegram does
func NewMessage(userID int64, chatID int64, chatType string, text string) *gotgbot.Message {
	msg := &gotgbot.Message{
		MessageId: atomic.AddInt64(&messageIDCounter, 1),
		From:      NewUser(userID),
		Chat:      gotgbot.Chat{Id: chatID, Type: chatType},
		Date:      1700000000,
		Text:      text,
	}
	if strings.HasPrefix(text, "/") {
		cmd := strings.Fields(text)[0]
		msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(cmd))}}
	}
	return msg
}

// MessageUpdate wraps a message into an update
func MessageUpdate(msg *gotgbot.Message) *gotgbot.Update {
	return &gotgbot.Update{UpdateId: atomic.AddInt64(&updateIDCounter, 1), Message: msg}
}

// EditedMessageUpdate wraps an edited message into an update
func EditedMessageUpdate(msg *gotgbot.Message) *gotgbot.Update {
	return &gotgbot.Update{UpdateId: atomic.AddInt64(&updateIDCounter, 1), EditedMessage: msg}
}

// CallbackUpdate builds a callback query update; the callback's message lives in the user's private chat
func CallbackUpdate(userID int64, data string) *gotgbot.Update {
	msg := NewPrivateMessage(TestBotID, "previous bot message")
	msg.Chat = gotgbot.Chat{Id: userID, Type: PrivateChatType}
	return &gotgbot.Update{
		UpdateId: atomic.AddInt64(&updateIDCounter, 1),
		CallbackQuery: &gotgbot.CallbackQuery{
			Id:   "cb-" + itoa(atomic.AddInt64(&updateIDCounter, 1)),
			From: *NewUser(userID),
			// ext.NewContext only recognizes a Message value (as decoded from real Bot API JSON), not a pointer
			Message: *msg,
			Data:    data,
		},
	}
}

// PollAnswerUpdate builds a poll answer update
func PollAnswerUpdate(userID int64, pollID string, optionIDs []int64) *gotgbot.Update {
	return &gotgbot.Update{
		UpdateId: atomic.AddInt64(&updateIDCounter, 1),
		PollAnswer: &gotgbot.PollAnswer{
			PollId:    pollID,
			User:      NewUser(userID),
			OptionIds: optionIDs,
		},
	}
}

// Dispatch runs a single handler against an update the same way ext.Dispatcher does.
// Reuse the same handler value across calls to keep conversation state.
// handled is false when the handler filter does not match the update.
func Dispatch(b *gotgbot.Bot, h ext.Handler, u *gotgbot.Update) (handled bool, err error) {
	ctx := ext.NewContext(b, u, nil)
	if !h.CheckUpdate(b, ctx) {
		return false, nil
	}
	return true, h.HandleUpdate(b, ctx)
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
