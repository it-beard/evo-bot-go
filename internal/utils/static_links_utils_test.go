package utils

import (
	"testing"

	"evo-bot-go/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestGetIntroMessageLink(t *testing.T) {
	tests := []struct {
		name            string
		config          *config.Config
		introMessageID  int64
		expectedURL     string
	}{
		{
			name: "Valid config and positive message ID",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     123,
			},
			introMessageID: 456,
			expectedURL:    "https://t.me/c/-1001234567890/123/456",
		},
		{
			name: "Large message ID",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     123,
			},
			introMessageID: 9223372036854775807, // max int64
			expectedURL:    "https://t.me/c/-1001234567890/123/9223372036854775807",
		},
		{
			name: "Zero message ID",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     123,
			},
			introMessageID: 0,
			expectedURL:    "https://t.me/c/-1001234567890/123/0",
		},
		{
			name: "Negative message ID",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     123,
			},
			introMessageID: -789,
			expectedURL:    "https://t.me/c/-1001234567890/123/-789",
		},
		{
			name: "Large chat ID and topic ID",
			config: &config.Config{
				SuperGroupChatID: -1009999999999,
				IntroTopicID:     999999,
			},
			introMessageID: 123456,
			expectedURL:    "https://t.me/c/-1009999999999/999999/123456",
		},
		{
			name: "Small positive values",
			config: &config.Config{
				SuperGroupChatID: -1001,
				IntroTopicID:     1,
			},
			introMessageID: 1,
			expectedURL:    "https://t.me/c/-1001/1/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIntroMessageLink(tt.config, tt.introMessageID)
			assert.Equal(t, tt.expectedURL, result, "URL should match expected format")
		})
	}
}

func TestGetIntroTopicLink(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectedURL string
	}{
		{
			name: "Valid config with standard values",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     123,
			},
			expectedURL: "https://t.me/c/-1001234567890/123",
		},
		{
			name: "Large chat ID",
			config: &config.Config{
				SuperGroupChatID: -1009999999999,
				IntroTopicID:     123,
			},
			expectedURL: "https://t.me/c/-1009999999999/123",
		},
		{
			name: "Large topic ID",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     999999,
			},
			expectedURL: "https://t.me/c/-1001234567890/999999",
		},
		{
			name: "Small positive values",
			config: &config.Config{
				SuperGroupChatID: -1001,
				IntroTopicID:     1,
			},
			expectedURL: "https://t.me/c/-1001/1",
		},
		{
			name: "Zero topic ID",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     0,
			},
			expectedURL: "https://t.me/c/-1001234567890/0",
		},
		{
			name: "Negative topic ID",
			config: &config.Config{
				SuperGroupChatID: -1001234567890,
				IntroTopicID:     -456,
			},
			expectedURL: "https://t.me/c/-1001234567890/-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIntroTopicLink(tt.config)
			assert.Equal(t, tt.expectedURL, result, "URL should match expected format")
		})
	}
}