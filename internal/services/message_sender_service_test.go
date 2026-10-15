package services

import (
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSenderFixture() (*MessageSenderService, *testutils.MockBotClient) {
	bot, client := testutils.NewTestBot()
	return NewMessageSenderService(bot), client
}

type sendFunc func(s *MessageSenderService, chatID int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error)

type senderVariant struct {
	name          string
	send          sendFunc
	returnsMsg    bool
	wantParseMode string
}

func senderVariants() []senderVariant {
	return []senderVariant{
		{
			name: "Send",
			send: func(s *MessageSenderService, c int64, txt string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				return nil, s.Send(c, txt, o)
			},
			returnsMsg: false,
		},
		{
			name:       "SendWithReturnMessage",
			send:       (*MessageSenderService).SendWithReturnMessage,
			returnsMsg: true,
		},
		{
			name: "SendMarkdown",
			send: func(s *MessageSenderService, c int64, txt string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				return nil, s.SendMarkdown(c, txt, o)
			},
			wantParseMode: "Markdown",
		},
		{
			name:          "SendMarkdownWithReturnMessage",
			send:          (*MessageSenderService).SendMarkdownWithReturnMessage,
			returnsMsg:    true,
			wantParseMode: "Markdown",
		},
		{
			name: "SendHtml",
			send: func(s *MessageSenderService, c int64, txt string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				return nil, s.SendHtml(c, txt, o)
			},
			wantParseMode: "HTML",
		},
		{
			name:          "SendHtmlWithReturnMessage",
			send:          (*MessageSenderService).SendHtmlWithReturnMessage,
			returnsMsg:    true,
			wantParseMode: "HTML",
		},
	}
}

func decodeLinkPreview(t *testing.T, call testutils.APICall) gotgbot.LinkPreviewOptions {
	t.Helper()
	var lp gotgbot.LinkPreviewOptions
	require.NoError(t, call.DecodeJSON("link_preview_options", &lp))
	return lp
}

func TestMessageSenderService_SendVariants_DefaultOptions(t *testing.T) {
	for _, v := range senderVariants() {
		t.Run(v.name, func(t *testing.T) {
			s, client := newSenderFixture()

			msg, err := v.send(s, -100500, "hello world", nil)

			require.NoError(t, err)
			if v.returnsMsg {
				require.NotNil(t, msg)
				assert.Equal(t, "hello world", msg.Text)
				assert.Equal(t, int64(-100500), msg.Chat.Id)
			}

			calls := client.CallsTo("sendMessage")
			require.Len(t, calls, 1)
			call := calls[0]
			assert.Equal(t, int64(-100500), call.Int64("chat_id"))
			assert.Equal(t, "hello world", call.Params["text"])
			assert.Equal(t, v.wantParseMode, call.Params["parse_mode"])
			assert.True(t, decodeLinkPreview(t, call).IsDisabled, "link previews must be disabled by default")
			_, hasThread := call.Params["message_thread_id"]
			assert.False(t, hasThread)
		})
	}
}

func TestMessageSenderService_SendVariants_CustomOptions(t *testing.T) {
	for _, v := range senderVariants() {
		t.Run(v.name+"/fills missing link preview and keeps thread", func(t *testing.T) {
			s, client := newSenderFixture()
			opts := &gotgbot.SendMessageOpts{MessageThreadId: 42, ParseMode: "MarkdownV2"}

			_, err := v.send(s, -1, "text", opts)

			require.NoError(t, err)
			call, ok := client.LastCallTo("sendMessage")
			require.True(t, ok)
			assert.Equal(t, int64(42), call.Int64("message_thread_id"))
			assert.True(t, decodeLinkPreview(t, call).IsDisabled)
			if v.wantParseMode == "" {
				assert.Equal(t, "MarkdownV2", call.Params["parse_mode"], "plain send keeps caller's parse mode")
			} else {
				assert.Equal(t, v.wantParseMode, call.Params["parse_mode"], "parse mode is forced by the variant")
			}
		})

		t.Run(v.name+"/keeps explicit link preview options", func(t *testing.T) {
			s, client := newSenderFixture()
			opts := &gotgbot.SendMessageOpts{
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: false, Url: "https://example.com"},
			}

			_, err := v.send(s, -1, "text", opts)

			require.NoError(t, err)
			call, ok := client.LastCallTo("sendMessage")
			require.True(t, ok)
			lp := decodeLinkPreview(t, call)
			assert.False(t, lp.IsDisabled)
			assert.Equal(t, "https://example.com", lp.Url)
		})
	}
}

func TestMessageSenderService_SendVariants_ErrorWithoutTopicReopen(t *testing.T) {
	for _, v := range senderVariants() {
		t.Run(v.name+"/generic error", func(t *testing.T) {
			s, client := newSenderFixture()
			client.OnError("sendMessage", "Bad Request: chat not found")

			msg, err := v.send(s, -1, "text", &gotgbot.SendMessageOpts{MessageThreadId: 3})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "chat not found")
			assert.Nil(t, msg)
			assert.Empty(t, client.CallsTo("reopenForumTopic"))
			assert.Len(t, client.CallsTo("sendMessage"), 1)
		})

		t.Run(v.name+"/topic closed without thread id", func(t *testing.T) {
			s, client := newSenderFixture()
			client.OnError("sendMessage", "Bad Request: TOPIC_CLOSED")

			_, err := v.send(s, -1, "text", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "TOPIC_CLOSED")
			assert.Empty(t, client.CallsTo("reopenForumTopic"))
			assert.Empty(t, client.CallsTo("closeForumTopic"))
		})
	}
}

// failFirstSendWith makes the first sendMessage call fail with the given description and the rest succeed
func failFirstSendWith(client *testutils.MockBotClient, description string) {
	var count int64
	client.On("sendMessage", func(call testutils.APICall) (json.RawMessage, error) {
		if atomic.AddInt64(&count, 1) == 1 {
			return nil, &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: description}
		}
		return json.Marshal(gotgbot.Message{
			MessageId:       555,
			Chat:            gotgbot.Chat{Id: call.Int64("chat_id"), Type: "supergroup"},
			Text:            call.Params["text"],
			MessageThreadId: call.Int64("message_thread_id"),
		})
	})
}

func TestMessageSenderService_SendVariants_TopicClosedReopensSendsAndCloses(t *testing.T) {
	for _, v := range senderVariants() {
		t.Run(v.name, func(t *testing.T) {
			s, client := newSenderFixture()
			failFirstSendWith(client, "Bad Request: TOPIC_CLOSED")

			msg, err := v.send(s, -100, "important", &gotgbot.SendMessageOpts{MessageThreadId: 9})

			require.NoError(t, err)
			if v.returnsMsg {
				require.NotNil(t, msg)
				assert.Equal(t, int64(555), msg.MessageId)
			}

			var methods []string
			for _, c := range client.Calls() {
				methods = append(methods, c.Method)
			}
			assert.Equal(t, []string{"sendMessage", "reopenForumTopic", "sendMessage", "closeForumTopic"}, methods)

			reopen, _ := client.LastCallTo("reopenForumTopic")
			assert.Equal(t, int64(-100), reopen.Int64("chat_id"))
			assert.Equal(t, int64(9), reopen.Int64("message_thread_id"))
			closeCall, _ := client.LastCallTo("closeForumTopic")
			assert.Equal(t, int64(-100), closeCall.Int64("chat_id"))
			assert.Equal(t, int64(9), closeCall.Int64("message_thread_id"))

			resend := client.CallsTo("sendMessage")[1]
			assert.Equal(t, "important", resend.Params["text"])
			assert.Equal(t, int64(9), resend.Int64("message_thread_id"))
			assert.Equal(t, v.wantParseMode, resend.Params["parse_mode"])
		})
	}
}

func TestMessageSenderService_SendWithReturnMessage_TopicClosedReopenFails(t *testing.T) {
	s, client := newSenderFixture()
	client.OnError("sendMessage", "Bad Request: TOPIC_CLOSED")
	client.OnError("reopenForumTopic", "Bad Request: not enough rights")

	msg, err := s.SendWithReturnMessage(-100, "text", &gotgbot.SendMessageOpts{MessageThreadId: 9})

	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Contains(t, err.Error(), "failed to reopen topic")
	var tgErr *gotgbot.TelegramError
	require.True(t, errors.As(err, &tgErr), "original error must be wrapped")
	assert.Contains(t, tgErr.Description, "TOPIC_CLOSED")
	assert.Len(t, client.CallsTo("sendMessage"), 1, "no resend when reopening fails")
	assert.Empty(t, client.CallsTo("closeForumTopic"))
}

func TestMessageSenderService_SendWithReturnMessage_TopicClosedResendFailsStillCloses(t *testing.T) {
	s, client := newSenderFixture()
	client.OnError("sendMessage", "Bad Request: TOPIC_CLOSED")

	msg, err := s.SendHtmlWithReturnMessage(-100, "text", &gotgbot.SendMessageOpts{MessageThreadId: 9})

	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Len(t, client.CallsTo("sendMessage"), 2)
	assert.Len(t, client.CallsTo("reopenForumTopic"), 1)
	assert.Len(t, client.CallsTo("closeForumTopic"), 1, "topic must be closed again even when resend fails")
}

func TestMessageSenderService_SendWithReturnMessage_TopicClosedCloseFailsStillSucceeds(t *testing.T) {
	s, client := newSenderFixture()
	failFirstSendWith(client, "Bad Request: TOPIC_CLOSED")
	client.OnError("closeForumTopic", "Bad Request: not enough rights")

	msg, err := s.SendMarkdownWithReturnMessage(-100, "text", &gotgbot.SendMessageOpts{MessageThreadId: 9})

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, int64(555), msg.MessageId)
	assert.Len(t, client.CallsTo("closeForumTopic"), 1)
}

func TestMessageSenderService_handleClosedTopic(t *testing.T) {
	s, client := newSenderFixture()
	original := errors.New("TOPIC_CLOSED")

	err := s.handleClosedTopic(-100, "text", &gotgbot.SendMessageOpts{MessageThreadId: 4}, "test", original)

	require.NoError(t, err)
	assert.Len(t, client.CallsTo("reopenForumTopic"), 1)
	assert.Len(t, client.CallsTo("sendMessage"), 1)
	assert.Len(t, client.CallsTo("closeForumTopic"), 1)

	client.OnError("reopenForumTopic", "Bad Request: nope")
	err = s.handleClosedTopic(-100, "text", &gotgbot.SendMessageOpts{MessageThreadId: 4}, "test", original)
	require.Error(t, err)
	assert.ErrorIs(t, err, original)
}

func TestMessageSenderService_isTopicClosedError(t *testing.T) {
	s, _ := newSenderFixture()

	assert.False(t, s.isTopicClosedError(nil))
	assert.False(t, s.isTopicClosedError(errors.New("Bad Request: chat not found")))
	assert.True(t, s.isTopicClosedError(errors.New("Bad Request: TOPIC_CLOSED")))
}

type replyVariant struct {
	name          string
	reply         func(s *MessageSenderService, msg *gotgbot.Message, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error)
	returnsMsg    bool
	wantParseMode string
}

func replyVariants() []replyVariant {
	return []replyVariant{
		{
			name: "Reply",
			reply: func(s *MessageSenderService, m *gotgbot.Message, txt string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				return nil, s.Reply(m, txt, o)
			},
		},
		{name: "ReplyWithReturnMessage", reply: (*MessageSenderService).ReplyWithReturnMessage, returnsMsg: true},
		{
			name: "ReplyMarkdown",
			reply: func(s *MessageSenderService, m *gotgbot.Message, txt string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				return nil, s.ReplyMarkdown(m, txt, o)
			},
			wantParseMode: "Markdown",
		},
		{name: "ReplyMarkdownWithReturnMessage", reply: (*MessageSenderService).ReplyMarkdownWithReturnMessage, returnsMsg: true, wantParseMode: "Markdown"},
		{
			name: "ReplyHtml",
			reply: func(s *MessageSenderService, m *gotgbot.Message, txt string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
				return nil, s.ReplyHtml(m, txt, o)
			},
			wantParseMode: "HTML",
		},
		{name: "ReplyHtmlWithReturnMessage", reply: (*MessageSenderService).ReplyHtmlWithReturnMessage, returnsMsg: true, wantParseMode: "HTML"},
	}
}

func TestMessageSenderService_ReplyVariants(t *testing.T) {
	for _, v := range replyVariants() {
		t.Run(v.name+"/default options", func(t *testing.T) {
			s, client := newSenderFixture()
			original := testutils.NewGroupMessage(1, -100777, 0, "question")

			msg, err := v.reply(s, original, "answer", nil)

			require.NoError(t, err)
			if v.returnsMsg {
				require.NotNil(t, msg)
				assert.Equal(t, "answer", msg.Text)
			}
			call, ok := client.LastCallTo("sendMessage")
			require.True(t, ok)
			assert.Equal(t, int64(-100777), call.Int64("chat_id"))
			assert.Equal(t, "answer", call.Params["text"])
			assert.Equal(t, v.wantParseMode, call.Params["parse_mode"])
			assert.True(t, decodeLinkPreview(t, call).IsDisabled)
			var rp gotgbot.ReplyParameters
			require.NoError(t, call.DecodeJSON("reply_parameters", &rp))
			assert.Equal(t, original.MessageId, rp.MessageId)
		})

		t.Run(v.name+"/custom options", func(t *testing.T) {
			s, client := newSenderFixture()
			original := testutils.NewPrivateMessage(5, "question")
			opts := &gotgbot.SendMessageOpts{
				ParseMode:          "MarkdownV2",
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: false, PreferLargeMedia: true},
			}

			_, err := v.reply(s, original, "answer", opts)

			require.NoError(t, err)
			call, _ := client.LastCallTo("sendMessage")
			lp := decodeLinkPreview(t, call)
			assert.False(t, lp.IsDisabled)
			assert.True(t, lp.PreferLargeMedia)
			if v.wantParseMode == "" {
				assert.Equal(t, "MarkdownV2", call.Params["parse_mode"])
			} else {
				assert.Equal(t, v.wantParseMode, call.Params["parse_mode"])
			}
		})

		t.Run(v.name+"/fills link preview when missing", func(t *testing.T) {
			s, client := newSenderFixture()
			original := testutils.NewPrivateMessage(5, "question")

			_, err := v.reply(s, original, "answer", &gotgbot.SendMessageOpts{})

			require.NoError(t, err)
			call, _ := client.LastCallTo("sendMessage")
			assert.True(t, decodeLinkPreview(t, call).IsDisabled)
		})

		t.Run(v.name+"/error", func(t *testing.T) {
			s, client := newSenderFixture()
			client.OnError("sendMessage", "Forbidden: bot was blocked by the user")
			original := testutils.NewPrivateMessage(5, "question")

			msg, err := v.reply(s, original, "answer", nil)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "blocked")
		})
	}
}

func TestMessageSenderService_ReplyWithCleanupAfterDelayWithPing_DeletesBothMessages(t *testing.T) {
	s, client := newSenderFixture()
	original := testutils.NewGroupMessage(31, -100999, 0, "/command")
	opts := &gotgbot.SendMessageOpts{ParseMode: "Markdown"}

	err := s.ReplyWithCleanupAfterDelayWithPing(original, "go to DM", 0, opts)

	require.NoError(t, err)
	sends := client.CallsTo("sendMessage")
	require.Len(t, sends, 2)

	reply := sends[0]
	assert.Equal(t, int64(-100999), reply.Int64("chat_id"))
	assert.Equal(t, "go to DM", reply.Params["text"])
	assert.Equal(t, "Markdown", reply.Params["parse_mode"])
	var rp gotgbot.ReplyParameters
	require.NoError(t, reply.DecodeJSON("reply_parameters", &rp))
	assert.Equal(t, original.MessageId, rp.MessageId)

	ping := sends[1]
	assert.Equal(t, int64(31), ping.Int64("chat_id"), "ping goes to the user's private chat")
	assert.Contains(t, []string{"Ping!", "Hi!", "Ку!", "Приветы!", "Дзень добры!", "Пинг!"}, ping.Params["text"])

	require.True(t, waitFor(t, 2*time.Second, func() bool { return len(client.CallsTo("deleteMessage")) == 2 }),
		"both messages must be deleted after the delay")
	deletes := client.CallsTo("deleteMessage")
	// The reply message id is assigned by the fake client; it differs from the original message id
	var deletedIDs []int64
	for _, d := range deletes {
		assert.Equal(t, int64(-100999), d.Int64("chat_id"))
		deletedIDs = append(deletedIDs, d.Int64("message_id"))
	}
	assert.Contains(t, deletedIDs, original.MessageId)
	assert.Len(t, deletedIDs, 2)
	assert.NotEqual(t, deletedIDs[0], deletedIDs[1])
	assert.Equal(t, original.MessageId, deletedIDs[1], "the original message is deleted after the reply")
}

func TestMessageSenderService_ReplyWithCleanupAfterDelayWithPing_ReplyFails(t *testing.T) {
	s, client := newSenderFixture()
	client.OnError("sendMessage", "Bad Request: chat not found")
	original := testutils.NewGroupMessage(31, -100999, 0, "/command")

	err := s.ReplyWithCleanupAfterDelayWithPing(original, "go to DM", 0, nil)

	require.Error(t, err)
	assert.Len(t, client.CallsTo("sendMessage"), 1, "no ping when the reply fails")
	// No cleanup goroutine is started when the reply fails, so nothing can be deleted
	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, client.CallsTo("deleteMessage"))
}

func TestMessageSenderService_ReplyWithCleanupAfterDelayWithPing_PingFailsStillCleansUp(t *testing.T) {
	s, client := newSenderFixture()
	original := testutils.NewGroupMessage(31, -100999, 0, "/command")
	client.On("sendMessage", func(call testutils.APICall) (json.RawMessage, error) {
		if call.Int64("chat_id") == 31 {
			return nil, &gotgbot.TelegramError{Method: "sendMessage", Code: 403, Description: "Forbidden: bot can't initiate conversation with a user"}
		}
		return json.Marshal(gotgbot.Message{MessageId: 9001, Chat: gotgbot.Chat{Id: call.Int64("chat_id"), Type: "supergroup"}})
	})

	_ = s.ReplyWithCleanupAfterDelayWithPing(original, "go to DM", 0, nil)

	require.True(t, waitFor(t, 2*time.Second, func() bool { return len(client.CallsTo("deleteMessage")) == 2 }))
	deletes := client.CallsTo("deleteMessage")
	assert.Equal(t, int64(9001), deletes[0].Int64("message_id"))
	assert.Equal(t, original.MessageId, deletes[1].Int64("message_id"))
}

func TestMessageSenderService_ReplyWithCleanupAfterDelayWithPing_DeleteErrorsAreTolerated(t *testing.T) {
	s, client := newSenderFixture()
	client.OnError("deleteMessage", "Bad Request: message to delete not found")
	original := testutils.NewGroupMessage(31, -100999, 0, "/command")

	err := s.ReplyWithCleanupAfterDelayWithPing(original, "go to DM", 0, nil)

	require.NoError(t, err)
	require.True(t, waitFor(t, 2*time.Second, func() bool { return len(client.CallsTo("deleteMessage")) == 2 }),
		"original message deletion is attempted even if deleting the reply fails")
}

func TestMessageSenderService_SendCopy_TextMessages(t *testing.T) {
	topic := 12
	entities := []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 4}}

	tests := []struct {
		name     string
		topicID  *int
		original *gotgbot.Message
	}{
		{name: "nil original without topic", topicID: nil, original: nil},
		{name: "nil original with topic", topicID: &topic, original: nil},
		{name: "text original with topic", topicID: &topic, original: testutils.NewPrivateMessage(1, "Bold text")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, client := newSenderFixture()

			msg, err := s.SendCopy(-100123, tt.topicID, "Bold text", entities, tt.original)

			require.NoError(t, err)
			require.NotNil(t, msg)
			calls := client.Calls()
			require.Len(t, calls, 1)
			call := calls[0]
			assert.Equal(t, "sendMessage", call.Method)
			assert.Equal(t, int64(-100123), call.Int64("chat_id"))
			assert.Equal(t, "Bold text", call.Params["text"])
			var gotEntities []gotgbot.MessageEntity
			require.NoError(t, call.DecodeJSON("entities", &gotEntities))
			assert.Equal(t, entities, gotEntities)
			if tt.topicID != nil {
				assert.Equal(t, int64(topic), call.Int64("message_thread_id"))
			} else {
				_, ok := call.Params["message_thread_id"]
				assert.False(t, ok)
			}
		})
	}
}

func TestMessageSenderService_SendCopy_Media(t *testing.T) {
	topic := 7
	entities := []gotgbot.MessageEntity{{Type: "italic", Offset: 0, Length: 5}}

	tests := []struct {
		name       string
		original   *gotgbot.Message
		wantMethod string
		fileParam  string
		wantFileID string
	}{
		{
			name:       "animation",
			original:   &gotgbot.Message{Animation: &gotgbot.Animation{FileId: "anim-1"}},
			wantMethod: "sendAnimation", fileParam: "animation", wantFileID: "anim-1",
		},
		{
			name: "photo uses the largest size",
			original: &gotgbot.Message{Photo: []gotgbot.PhotoSize{
				{FileId: "photo-small", Width: 90}, {FileId: "photo-large", Width: 1280},
			}},
			wantMethod: "sendPhoto", fileParam: "photo", wantFileID: "photo-large",
		},
		{
			name:       "video",
			original:   &gotgbot.Message{Video: &gotgbot.Video{FileId: "video-1"}},
			wantMethod: "sendVideo", fileParam: "video", wantFileID: "video-1",
		},
	}

	for _, tt := range tests {
		for _, withTopic := range []bool{false, true} {
			name := tt.name
			var topicID *int
			if withTopic {
				name += " in topic"
				topicID = &topic
			}
			t.Run(name, func(t *testing.T) {
				s, client := newSenderFixture()

				msg, err := s.SendCopy(-100123, topicID, "Short caption", entities, tt.original)

				require.NoError(t, err)
				require.NotNil(t, msg)
				calls := client.Calls()
				require.Len(t, calls, 1, "short captions are sent in a single request")
				call := calls[0]
				assert.Equal(t, tt.wantMethod, call.Method)
				assert.Equal(t, int64(-100123), call.Int64("chat_id"))
				assert.Equal(t, tt.wantFileID, call.Params[tt.fileParam])
				assert.Equal(t, "Short caption", call.Params["caption"])
				var gotEntities []gotgbot.MessageEntity
				require.NoError(t, call.DecodeJSON("caption_entities", &gotEntities))
				assert.Equal(t, entities, gotEntities)
				if withTopic {
					assert.Equal(t, int64(topic), call.Int64("message_thread_id"))
				} else {
					_, ok := call.Params["message_thread_id"]
					assert.False(t, ok)
				}
			})
		}
	}
}

func TestMessageSenderService_SendCopy_LongCaptionIsSplit(t *testing.T) {
	head := strings.Repeat("a", 990)
	tail := strings.Repeat("b", 300)
	text := head + "LINKTEXT" + tail // the cut at 996 UTF-16 units lands inside "LINKTEXT"
	require.Equal(t, 1298, len(text))

	before := gotgbot.MessageEntity{Type: "bold", Offset: 10, Length: 20}
	after := gotgbot.MessageEntity{Type: "text_link", Offset: 1100, Length: 50, Url: "https://example.com"}
	entities := []gotgbot.MessageEntity{before, after}
	original := &gotgbot.Message{
		Photo:           []gotgbot.PhotoSize{{FileId: "photo-1"}},
		Caption:         text,
		CaptionEntities: entities,
	}
	topic := 3

	s, client := newSenderFixture()
	msg, err := s.SendCopy(-100123, &topic, text, entities, original)

	require.NoError(t, err)
	require.NotNil(t, msg)
	calls := client.Calls()
	require.Len(t, calls, 2)

	photo := calls[0]
	assert.Equal(t, "sendPhoto", photo.Method)
	assert.Equal(t, text[:996], photo.Params["caption"])
	assert.Equal(t, int64(topic), photo.Int64("message_thread_id"))
	var captionEntities []gotgbot.MessageEntity
	require.NoError(t, photo.DecodeJSON("caption_entities", &captionEntities))
	assert.Equal(t, []gotgbot.MessageEntity{before}, captionEntities, "only entities within the caption are kept")

	rest := calls[1]
	assert.Equal(t, "sendMessage", rest.Method)
	assert.Equal(t, int64(-100123), rest.Int64("chat_id"))
	assert.Equal(t, "..."+text[996:], rest.Params["text"])
	var restEntities []gotgbot.MessageEntity
	require.NoError(t, rest.DecodeJSON("entities", &restEntities))
	require.Len(t, restEntities, 1)
	assert.Equal(t, "text_link", restEntities[0].Type)
	assert.Equal(t, int64(1100-996+3), restEntities[0].Offset, "offset is shifted to the trimmed part plus the ellipsis")
	assert.Equal(t, int64(50), restEntities[0].Length)
	assert.Equal(t, "https://example.com", restEntities[0].Url)
	// Sanity check: the entity points at the same characters in the trimmed message
	restText := rest.Params["text"]
	assert.Equal(t, text[1100:1150], restText[restEntities[0].Offset:restEntities[0].Offset+50])
}

func TestMessageSenderService_SendCopy_LongCaptionClipsSpanningEntityInCaption(t *testing.T) {
	text := strings.Repeat("x", 1200)
	spanning := gotgbot.MessageEntity{Type: "italic", Offset: 990, Length: 20}
	original := &gotgbot.Message{
		Video:           &gotgbot.Video{FileId: "video-1"},
		CaptionEntities: []gotgbot.MessageEntity{spanning},
	}

	s, client := newSenderFixture()
	_, err := s.SendCopy(-100123, nil, text, original.CaptionEntities, original)

	require.NoError(t, err)
	video, ok := client.LastCallTo("sendVideo")
	require.True(t, ok)
	var captionEntities []gotgbot.MessageEntity
	require.NoError(t, video.DecodeJSON("caption_entities", &captionEntities))
	require.Len(t, captionEntities, 1)
	assert.Equal(t, int64(990), captionEntities[0].Offset)
	assert.Equal(t, int64(6), captionEntities[0].Length, "entity is clipped at the 996 unit cutoff")
	assert.Len(t, client.CallsTo("sendMessage"), 1, "the remainder is sent as a separate message")
}

func TestMessageSenderService_SendCopy_LongCaptionCountsUTF16Units(t *testing.T) {
	// Each emoji takes 2 UTF-16 code units: 600 emojis = 1200 units > 1000
	text := strings.Repeat("😀", 600)
	original := &gotgbot.Message{Animation: &gotgbot.Animation{FileId: "anim-1"}}

	s, client := newSenderFixture()
	_, err := s.SendCopy(-1, nil, text, nil, original)

	require.NoError(t, err)
	anim, ok := client.LastCallTo("sendAnimation")
	require.True(t, ok)
	assert.Equal(t, strings.Repeat("😀", 498), anim.Params["caption"])
	rest, ok := client.LastCallTo("sendMessage")
	require.True(t, ok)
	assert.Equal(t, "..."+strings.Repeat("😀", 102), rest.Params["text"])
}

func TestMessageSenderService_SendCopy_LongTextMessageIsNotSplit(t *testing.T) {
	text := strings.Repeat("t", 1500)
	original := testutils.NewPrivateMessage(1, text)

	s, client := newSenderFixture()
	_, err := s.SendCopy(-1, nil, text, nil, original)

	require.NoError(t, err)
	calls := client.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "sendMessage", calls[0].Method)
	assert.Equal(t, text, calls[0].Params["text"])
}

func TestMessageSenderService_SendCopy_Errors(t *testing.T) {
	t.Run("media send fails", func(t *testing.T) {
		s, client := newSenderFixture()
		client.OnError("sendPhoto", "Bad Request: wrong file identifier")
		original := &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "p"}}}

		msg, err := s.SendCopy(-1, nil, strings.Repeat("a", 1100), nil, original)

		require.Error(t, err)
		assert.Nil(t, msg)
		assert.Empty(t, client.CallsTo("sendMessage"), "remainder is not sent when the media fails")
	})

	t.Run("text send fails", func(t *testing.T) {
		s, client := newSenderFixture()
		client.OnError("sendMessage", "Bad Request: chat not found")

		msg, err := s.SendCopy(-1, nil, "text", nil, nil)

		require.Error(t, err)
		assert.Nil(t, msg)
	})

	t.Run("remainder send fails", func(t *testing.T) {
		s, client := newSenderFixture()
		client.OnError("sendMessage", "Bad Request: chat not found")
		original := &gotgbot.Message{Animation: &gotgbot.Animation{FileId: "a"}}

		msg, err := s.SendCopy(-1, nil, strings.Repeat("a", 1100), nil, original)

		require.Error(t, err)
		require.NotNil(t, msg, "the media message is returned even if the remainder fails")
		assert.Len(t, client.CallsTo("sendAnimation"), 1)
	})
}

func TestMessageSenderService_SendTypingAction(t *testing.T) {
	s, client := newSenderFixture()

	require.NoError(t, s.SendTypingAction(4242))

	call, ok := client.LastCallTo("sendChatAction")
	require.True(t, ok)
	assert.Equal(t, int64(4242), call.Int64("chat_id"))
	assert.Equal(t, "typing", call.Params["action"])

	client.OnError("sendChatAction", "Bad Request: chat not found")
	assert.Error(t, s.SendTypingAction(4242))
}

func TestMessageSenderService_RemoveInlineKeyboard(t *testing.T) {
	t.Run("removes keyboard", func(t *testing.T) {
		s, client := newSenderFixture()

		require.NoError(t, s.RemoveInlineKeyboard(10, 20))

		call, ok := client.LastCallTo("editMessageReplyMarkup")
		require.True(t, ok)
		assert.Equal(t, int64(10), call.Int64("chat_id"))
		assert.Equal(t, int64(20), call.Int64("message_id"))
		var markup gotgbot.InlineKeyboardMarkup
		require.NoError(t, call.DecodeJSON("reply_markup", &markup))
		assert.Empty(t, markup.InlineKeyboard)
	})

	t.Run("missing ids are a no-op", func(t *testing.T) {
		s, client := newSenderFixture()

		assert.NoError(t, s.RemoveInlineKeyboard(0, 20))
		assert.NoError(t, s.RemoveInlineKeyboard(10, 0))
		assert.Empty(t, client.Calls())
	})

	t.Run("error is returned", func(t *testing.T) {
		s, client := newSenderFixture()
		client.OnError("editMessageReplyMarkup", "Bad Request: message is not modified")

		assert.Error(t, s.RemoveInlineKeyboard(10, 20))
	})
}

func TestMessageSenderService_PinMessage(t *testing.T) {
	tests := []struct {
		name             string
		withNotification bool
		wantDisable      string
	}{
		{name: "with notification", withNotification: true, wantDisable: "false"},
		{name: "silent", withNotification: false, wantDisable: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, client := newSenderFixture()

			require.NoError(t, s.PinMessage(-100, 77, tt.withNotification))

			call, ok := client.LastCallTo("pinChatMessage")
			require.True(t, ok)
			assert.Equal(t, int64(-100), call.Int64("chat_id"))
			assert.Equal(t, int64(77), call.Int64("message_id"))
			assert.Equal(t, tt.wantDisable, call.Params["disable_notification"])
		})
	}

	t.Run("error", func(t *testing.T) {
		s, client := newSenderFixture()
		client.OnError("pinChatMessage", "Bad Request: not enough rights")

		assert.Error(t, s.PinMessage(-100, 77, true))
	})
}
