package clients

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/storage"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

var (
	verificationCode string
	codeMutex        sync.RWMutex
	sessionStorage   session.Storage
)

func init() {
	// Initialize session storage based on configuration
	// Load configuration
	appConfig, err := config.LoadConfig()
	if err != nil {
		log.Printf("Failed to load configuration, using in-memory session storage: %v", err)
		sessionStorage = new(session.StorageMemory)
		return
	}

	sessionType := strings.ToLower(appConfig.TGUserClientSessionType)

	switch sessionType {
	case constants.TGUserClientSessionTypeFile:
		sessionStorage = &session.FileStorage{Path: constants.TGUserClientDefaultSessionFile}
		log.Printf("Using file session storage (%s)", constants.TGUserClientDefaultSessionFile)
	case constants.TGUserClientSessionTypeDatabase:
		// Initialize database storage
		dbSessionStorage, err := initDatabaseStorage(appConfig.DBConnection)
		if err != nil {
			log.Printf("Failed to initialize database session storage: %v, falling back to in-memory storage", err)
			sessionStorage = new(session.StorageMemory)
		} else {
			sessionStorage = dbSessionStorage
			log.Print("Using database session storage")
		}
	default:
		sessionStorage = new(session.StorageMemory)
		log.Print("Using in-memory session storage")
	}
}

// initDatabaseStorage initializes a database-backed session storage
func initDatabaseStorage(connectionString string) (session.Storage, error) {
	if connectionString == "" {
		return nil, fmt.Errorf("database connection string is empty")
	}

	// Create database connection
	db, err := storage.NewDB(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create session store
	sessionStore, err := storage.NewSessionStore(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	return sessionStore, nil
}

// TelegramConfig holds the configuration for Telegram client
type TelegramConfig struct {
	AppID       int
	AppHash     string
	PhoneNumber string
	Password    string
}

// NewTelegramConfig creates a new TelegramConfig from config values
func NewTelegramConfig() (*TelegramConfig, error) {
	// Load configuration
	appConfig, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appID := appConfig.TGUserClientAppID
	appHash := appConfig.TGUserClientAppHash
	phoneNumber := appConfig.TGUserClientPhoneNumber
	password := appConfig.TGUserClient2FAPass

	if appID == 0 || appHash == "" || phoneNumber == "" || password == "" {
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
		SessionStorage: sessionStorage,
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
				Limit:   constants.TGUserClientDefaultLimit,
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
	limit := constants.TGUserClientDefaultLimit

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
		Limit:      constants.TGUserClientDefaultLimit,
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
