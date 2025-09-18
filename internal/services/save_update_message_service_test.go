package services

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
)

func TestSaveUpdateMessageService_extractMessageTextAndEntities(t *testing.T) {
	service := &SaveUpdateMessageService{}

	t.Run("Text message with entities", func(t *testing.T) {
		msg := &gotgbot.Message{
			Text: "Hello world",
			Entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
			},
		}
		text, entities := service.extractMessageTextAndEntities(msg)
		assert.Equal(t, "Hello world", text)
		assert.Len(t, entities, 1)
	})

	t.Run("Photo message without caption", func(t *testing.T) {
		msg := &gotgbot.Message{
			Photo: []gotgbot.PhotoSize{{FileId: "test"}},
		}
		text, entities := service.extractMessageTextAndEntities(msg)
		assert.Equal(t, "[Photo]", text)
		assert.Len(t, entities, 0)
	})

	t.Run("Caption message", func(t *testing.T) {
		msg := &gotgbot.Message{
			Caption: "Photo caption",
			CaptionEntities: []gotgbot.MessageEntity{
				{Type: "italic", Offset: 0, Length: 5},
			},
		}
		text, entities := service.extractMessageTextAndEntities(msg)
		assert.Equal(t, "Photo caption", text)
		assert.Len(t, entities, 1)
	})

	t.Run("Empty message", func(t *testing.T) {
		msg := &gotgbot.Message{}
		text, entities := service.extractMessageTextAndEntities(msg)
		assert.Equal(t, "[Media]", text)
		assert.Len(t, entities, 0)
	})
}

func TestSaveUpdateMessageService_getMediaTypeDescription(t *testing.T) {
	service := &SaveUpdateMessageService{}

	tests := []struct {
		name     string
		msg      *gotgbot.Message
		expected string
	}{
		{"Photo", &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "test"}}}, "[Photo]"},
		{"Video", &gotgbot.Message{Video: &gotgbot.Video{FileId: "test"}}, "[Video]"},
		{"Voice", &gotgbot.Message{Voice: &gotgbot.Voice{FileId: "test"}}, "[Voice message]"},
		{"Audio", &gotgbot.Message{Audio: &gotgbot.Audio{FileId: "test"}}, "[Audio]"},
		{"Document", &gotgbot.Message{Document: &gotgbot.Document{FileId: "test"}}, "[Document]"},
		{"VideoNote", &gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "test"}}, "[Video note]"},
		{"Sticker", &gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "test"}}, "[Sticker]"},
		{"Animation", &gotgbot.Message{Animation: &gotgbot.Animation{FileId: "test"}}, "[GIF]"},
		{"Unknown", &gotgbot.Message{}, "[Media]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getMediaTypeDescription(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSaveUpdateMessageService_extractReplyToMessageID(t *testing.T) {
	service := &SaveUpdateMessageService{}

	t.Run("Message with reply", func(t *testing.T) {
		msg := &gotgbot.Message{
			ReplyToMessage: &gotgbot.Message{MessageId: 123},
		}
		result := service.extractReplyToMessageID(msg)
		assert.NotNil(t, result)
		assert.Equal(t, int64(123), *result)
	})

	t.Run("Message without reply", func(t *testing.T) {
		msg := &gotgbot.Message{}
		result := service.extractReplyToMessageID(msg)
		assert.Nil(t, result)
	})
}

func TestSaveUpdateMessageService_extractGroupTopicID(t *testing.T) {
	service := &SaveUpdateMessageService{}

	tests := []struct {
		name     string
		msg      *gotgbot.Message
		expected int64
	}{
		{
			"Topic message with thread ID",
			&gotgbot.Message{MessageThreadId: 456, IsTopicMessage: true},
			456,
		},
		{
			"Non-topic message with thread ID",
			&gotgbot.Message{MessageThreadId: 456, IsTopicMessage: false},
			0,
		},
		{
			"Message without thread ID",
			&gotgbot.Message{MessageThreadId: 0, IsTopicMessage: true},
			0,
		},
		{
			"Regular message",
			&gotgbot.Message{},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.extractGroupTopicID(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSaveUpdateMessageService_extractAndFormatMessageContent(t *testing.T) {
	service := &SaveUpdateMessageService{}

	t.Run("Text message with formatting", func(t *testing.T) {
		msg := &gotgbot.Message{
			Text: "Hello world",
			Entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
			},
		}
		result := service.extractAndFormatMessageContent(msg)
		// The actual HTML conversion is handled by utils.ConvertToHTML
		// We just verify that the method doesn't panic and returns a string
		assert.NotEmpty(t, result)
	})

	t.Run("Photo message without caption", func(t *testing.T) {
		msg := &gotgbot.Message{
			Photo: []gotgbot.PhotoSize{{FileId: "test"}},
		}
		result := service.extractAndFormatMessageContent(msg)
		assert.NotEmpty(t, result)
	})

	t.Run("Caption message with entities", func(t *testing.T) {
		msg := &gotgbot.Message{
			Caption: "Photo caption",
			CaptionEntities: []gotgbot.MessageEntity{
				{Type: "italic", Offset: 0, Length: 5},
			},
		}
		result := service.extractAndFormatMessageContent(msg)
		assert.NotEmpty(t, result)
	})
}

func TestSaveUpdateMessageService_getMessageText(t *testing.T) {
	service := &SaveUpdateMessageService{}

	t.Run("Text message", func(t *testing.T) {
		msg := &gotgbot.Message{
			Text: "Hello world",
		}
		result := service.getMessageText(msg)
		assert.Equal(t, "Hello world", result)
	})

	t.Run("Caption message", func(t *testing.T) {
		msg := &gotgbot.Message{
			Caption: "Photo caption",
		}
		result := service.getMessageText(msg)
		assert.Equal(t, "Photo caption", result)
	})

	t.Run("Media message without caption", func(t *testing.T) {
		msg := &gotgbot.Message{
			Photo: []gotgbot.PhotoSize{{FileId: "test"}},
		}
		result := service.getMessageText(msg)
		assert.Equal(t, "[Photo]", result)
	})

	t.Run("Empty message", func(t *testing.T) {
		msg := &gotgbot.Message{}
		result := service.getMessageText(msg)
		assert.Equal(t, "[Media]", result)
	})

	t.Run("Video message without caption", func(t *testing.T) {
		msg := &gotgbot.Message{
			Video: &gotgbot.Video{FileId: "test"},
		}
		result := service.getMessageText(msg)
		assert.Equal(t, "[Video]", result)
	})

	t.Run("Voice message", func(t *testing.T) {
		msg := &gotgbot.Message{
			Voice: &gotgbot.Voice{FileId: "test"},
		}
		result := service.getMessageText(msg)
		assert.Equal(t, "[Voice message]", result)
	})

	t.Run("Document message", func(t *testing.T) {
		msg := &gotgbot.Message{
			Document: &gotgbot.Document{FileId: "test"},
		}
		result := service.getMessageText(msg)
		assert.Equal(t, "[Document]", result)
	})

	t.Run("Sticker message", func(t *testing.T) {
		msg := &gotgbot.Message{
			Sticker: &gotgbot.Sticker{FileId: "test"},
		}
		result := service.getMessageText(msg)
		assert.Equal(t, "[Sticker]", result)
	})
}
