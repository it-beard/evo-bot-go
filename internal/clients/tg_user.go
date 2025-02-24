package clients

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// Environment variable names
const (
	envSessionType = "TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE"
	envAppID       = "TG_EVO_BOT_TGUSERCLIENT_APPID"
	envAppHash     = "TG_EVO_BOT_TGUSERCLIENT_APPHASH"
	envPhoneNumber = "TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER"
	envPassword    = "TG_EVO_BOT_TGUSERCLIENT_2FAPASS"

	defaultSessionFile = "session.json"
	defaultLimit       = 100
)

var (
	verificationCode string
	codeMutex        sync.RWMutex
	storage          session.Storage
)

func init() {
	// Initialize session storage based on environment configuration
	sessionType := os.Getenv(envSessionType)
	if strings.ToLower(sessionType) == "file" {
		storage = &session.FileStorage{Path: defaultSessionFile}
		log.Printf("Using file session storage (%s)", defaultSessionFile)
	} else {
		storage = new(session.StorageMemory)
		log.Print("Using in-memory session storage")
	}
}

// TelegramConfig holds the configuration for Telegram client
type TelegramConfig struct {
	AppID       int
	AppHash     string
	PhoneNumber string
	Password    string
}

// NewTelegramConfig creates a new TelegramConfig from environment variables
func NewTelegramConfig() (*TelegramConfig, error) {
	appIDStr := os.Getenv(envAppID)
	appHash := os.Getenv(envAppHash)
	phoneNumber := os.Getenv(envPhoneNumber)
	password := os.Getenv(envPassword)

	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid app ID: %w", err)
	}

	if appHash == "" || phoneNumber == "" || password == "" {
		return nil, fmt.Errorf("missing required telegram client configuration")
	}

	return &TelegramConfig{
		AppID:       appID,
		AppHash:     appHash,
		PhoneNumber: phoneNumber,
		Password:    password,
	}, nil
}

// TelegramClient wraps the Telegram client functionality
type TelegramClient struct {
	client *telegram.Client
	config *TelegramConfig
}

// NewTelegramClient creates a new TelegramClient
func NewTelegramClient() (*TelegramClient, error) {
	config, err := NewTelegramConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram config: %w", err)
	}

	client := telegram.NewClient(config.AppID, config.AppHash, telegram.Options{
		SessionStorage: storage,
	})

	return &TelegramClient{
		client: client,
		config: config,
	}, nil
}

// SetVerificationCode sets the verification code for authentication
func SetVerificationCode(code string) {
	codeMutex.Lock()
	defer codeMutex.Unlock()
	verificationCode = code
}

// GetChatMessageById retrieves a specific message by its ID from a chat
// For forum topics, it returns a synthetic message with the topic name
func GetChatMessageById(chatID int64, messageID int) (*tg.Message, error) {
	tgClient, err := NewTelegramClient()
	if err != nil {
		return nil, err
	}

	var message *tg.Message
	err = tgClient.client.Run(context.Background(), func(ctx context.Context) error {
		if err := tgClient.ensureAuthorized(ctx); err != nil {
			return err
		}

		api := tgClient.client.API()
		inputPeer, err := tgClient.getPeerInfoByChatID(ctx, chatID)
		if err != nil {
			return fmt.Errorf("failed to get peer info: %w", err)
		}

		// For channels, try to get forum topics directly
		if channel, ok := inputPeer.(*tg.InputPeerChannel); ok {
			inputChannel := &tg.InputChannel{
				ChannelID:  channel.ChannelID,
				AccessHash: channel.AccessHash,
			}

			// Try to get forum topics
			forumTopics, err := api.ChannelsGetForumTopics(ctx, &tg.ChannelsGetForumTopicsRequest{
				Channel: inputChannel,
				Limit:   defaultLimit,
			})
			if err == nil {
				for _, topic := range forumTopics.Topics {
					if forumTopic, ok := topic.(*tg.ForumTopic); ok {
						if forumTopic.ID == messageID {
							// Create a synthetic message with the topic name
							message = &tg.Message{
								ID:      messageID,
								Message: forumTopic.Title,
							}
							return nil
						}
					}
				}
			}
		}

		return fmt.Errorf("message not found or unexpected response type")
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	if message == nil {
		return nil, fmt.Errorf("message not found")
	}

	return message, nil
}

// GetChatMessages retrieves messages from a chat topic
func GetChatMessages(chatID int64, topicID int) ([]tg.Message, error) {
	tgClient, err := NewTelegramClient()
	if err != nil {
		return nil, err
	}

	var allMessages []tg.Message
	err = tgClient.client.Run(context.Background(), func(ctx context.Context) error {
		if err := tgClient.ensureAuthorized(ctx); err != nil {
			return err
		}

		api := tgClient.client.API()
		inputPeer, err := tgClient.getPeerInfoByChatID(ctx, chatID)
		if err != nil {
			return fmt.Errorf("failed to get peer info: %w", err)
		}

		return tgClient.fetchMessages(ctx, api, inputPeer, topicID, &allMessages)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	return allMessages, nil
}

// fetchMessages retrieves messages with pagination
func (t *TelegramClient) fetchMessages(ctx context.Context, api *tg.Client, inputPeer tg.InputPeerClass, topicID int, allMessages *[]tg.Message) error {
	offset := 0
	limit := defaultLimit

	for {
		// Get messages from chat with pagination
		resp, err := api.MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{
			Peer:      inputPeer,
			Limit:     limit,
			MsgID:     topicID,
			AddOffset: offset,
		})
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		// Extract messages from response
		batchMessages, err := extractMessages(resp)
		if err != nil {
			return err
		}

		// If no messages returned, we've reached the end
		if len(batchMessages) == 0 {
			break
		}

		// Append batch messages to all messages
		*allMessages = append(*allMessages, batchMessages...)

		// If we got less messages than the limit, we've reached the end
		if len(batchMessages) < limit {
			break
		}

		// Increment offset for next batch
		offset += limit
	}

	return nil
}

// extractMessages extracts messages from the API response
func extractMessages(resp tg.MessagesMessagesClass) ([]tg.Message, error) {
	var messages []tg.Message

	switch m := resp.(type) {
	case *tg.MessagesChannelMessages:
		for _, msg := range m.Messages {
			if message, ok := msg.(*tg.Message); ok {
				messages = append(messages, *message)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	return messages, nil
}

// KeepSessionAlive keeps the Telegram session alive
func KeepSessionAlive() error {
	tgClient, err := NewTelegramClient()
	if err != nil {
		return err
	}

	err = tgClient.client.Run(context.Background(), func(ctx context.Context) error {
		if err := tgClient.ensureAuthorized(ctx); err != nil {
			return err
		}

		// Make a simple request to keep session alive
		_, err := tgClient.client.API().HelpGetConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to keep session alive: %w", err)
	}

	return nil
}

// getPeerInfoByChatID retrieves peer information by chat ID
func (t *TelegramClient) getPeerInfoByChatID(ctx context.Context, chatID int64) (tg.InputPeerClass, error) {
	api := t.client.API()

	// Fetch dialogs to find the chat and get its AccessHash if needed
	dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		Limit:      defaultLimit,
		OffsetPeer: &tg.InputPeerChat{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dialogs: %w", err)
	}

	switch dlg := dialogs.(type) {
	case *tg.MessagesDialogsSlice:
		for _, chat := range dlg.Chats {
			switch c := chat.(type) {
			case *tg.Chat:
				if c.ID == chatID {
					return &tg.InputPeerChat{ChatID: c.ID}, nil
				}
			case *tg.Channel:
				if c.ID == chatID {
					return &tg.InputPeerChannel{
						ChannelID:  c.ID,
						AccessHash: c.AccessHash,
					}, nil
				}
			}
		}
	default:
		return nil, fmt.Errorf("unexpected response type: %T", dialogs)
	}

	return nil, fmt.Errorf("chat with ID %d not found", chatID)
}

// ensureAuthorized ensures the client is authorized
func (t *TelegramClient) ensureAuthorized(ctx context.Context) error {
	authCli := t.client.Auth()

	status, err := authCli.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth status: %w", err)
	}

	if !status.Authorized {
		code, err := authCli.SendCode(ctx, t.config.PhoneNumber, auth.SendCodeOptions{
			AllowAppHash: true,
		})
		if err != nil {
			return fmt.Errorf("failed to send code: %w", err)
		}
		sentCode := code.(*tg.AuthSentCode)

		codeMutex.RLock()
		receivedCode := verificationCode
		codeMutex.RUnlock()

		if receivedCode == "" {
			return fmt.Errorf("verification code not set - use /code command to set it")
		}
		log.Print("Code applied")

		_, err = authCli.SignIn(ctx, t.config.PhoneNumber, receivedCode, sentCode.PhoneCodeHash)
		if err != nil {
			if strings.Contains(err.Error(), "2FA required") {
				_, err = authCli.Password(ctx, t.config.Password)
				if err != nil {
					return fmt.Errorf("failed to send 2FA password: %w", err)
				}
			} else {
				return fmt.Errorf("sign in error: %w", err)
			}
		}
	}

	return nil
}
