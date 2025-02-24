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

var (
	verificationCode string
	codeMutex        sync.RWMutex
	storage          session.Storage
)

func init() {
	// Read the session type from the environment.
	// If set to "file" (case-insensitive), use file storage ("session.json");
	// otherwise, default to in-memory storage.
	sessionType := os.Getenv("TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE")
	if strings.ToLower(sessionType) == "file" {
		// Get session file from environment variable; default to "@TG"
		sessionFile := "session.json"
		storage = &session.FileStorage{Path: sessionFile}
		log.Printf("Using file session storage (%s)", sessionFile)
	} else {
		// Default: in-memory session storage.
		storage = new(session.StorageMemory)
		log.Print("Using in-memory session storage")
	}
}

type TelegramUserClientConfig struct {
	appId       int
	appHash     string
	phoneNumber string
	password    string
}

func NewTelegramUserClientConfig() (*TelegramUserClientConfig, error) {
	// Parse environment variables
	appIdStr := os.Getenv("TG_EVO_BOT_TGUSERCLIENT_APPID")
	appHash := os.Getenv("TG_EVO_BOT_TGUSERCLIENT_APPHASH")
	phoneNumber := os.Getenv("TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER")
	password := os.Getenv("TG_EVO_BOT_TGUSERCLIENT_2FAPASS")

	// Convert appId to int
	appId, err := strconv.Atoi(appIdStr)
	if err != nil {
		return nil, fmt.Errorf("invalid TG_EVO_BOT_TGUSERCLIENT_APPID: %w", err)
	}

	// Validate required fields
	if appHash == "" || phoneNumber == "" || password == "" {
		return nil, fmt.Errorf("missing required telegram client configuration")
	}

	return &TelegramUserClientConfig{
		appId:       appId,
		appHash:     appHash,
		phoneNumber: phoneNumber,
		password:    password,
	}, nil
}

// GetChatMessageById retrieves a specific message by its ID from a chat
// For forum topics, it returns a synthetic message with the topic name
func GetChatMessageById(chatId int64, messageId int) (*tg.Message, error) {
	config, err := NewTelegramUserClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram config: %w", err)
	}

	var message *tg.Message
	client := telegram.NewClient(config.appId, config.appHash, telegram.Options{SessionStorage: storage})

	err = client.Run(context.Background(), func(ctx context.Context) error {
		if err := ensureAuthorized(ctx, client, config.phoneNumber, config.password); err != nil {
			return err
		}

		api := client.API()
		inputPeer, err := getPeerInfoByChatId(chatId, client, ctx)
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
				Limit:   100,
			})
			if err == nil {
				for _, topic := range forumTopics.Topics {
					if forumTopic, ok := topic.(*tg.ForumTopic); ok {
						if forumTopic.ID == messageId {
							// Create a synthetic message with the topic name
							message = &tg.Message{
								ID:      messageId,
								Message: forumTopic.Title,
							}
							return nil
						}
					}
				}
			}
		}

		// If we still couldn't find the message, return an error
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

func GetChatMessagesNew(chatId int64, topicId int) ([]tg.Message, error) {
	var allMessages []tg.Message

	// Get config from environment
	config, err := NewTelegramUserClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram config: %w", err)
	}

	client := telegram.NewClient(config.appId, config.appHash, telegram.Options{SessionStorage: storage})

	err = client.Run(context.Background(), func(ctx context.Context) error {
		// Ensure we're authorized
		if err := ensureAuthorized(ctx, client, config.phoneNumber, config.password); err != nil {
			return err
		}

		// start getting messages
		api := client.API()

		// Get peer info by chat id
		inputPeer, err := getPeerInfoByChatId(chatId, client, ctx)
		if err != nil {
			return fmt.Errorf("failed to get peer info: %w", err)
		}

		offset := 0
		limit := 100
		for {
			// Get messages from chat with pagination
			resp, err := api.MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{
				Peer:      inputPeer,
				Limit:     limit,
				MsgID:     topicId,
				AddOffset: offset, // Add offset for pagination
			})
			if err != nil {
				return fmt.Errorf("failed to get messages: %w", err)
			}

			// Extract messages from response
			var batchMessages []tg.Message
			switch m := resp.(type) {
			case *tg.MessagesChannelMessages:
				for _, msg := range m.Messages {
					if message, ok := msg.(*tg.Message); ok {
						batchMessages = append(batchMessages, *message)
					}
				}
			default:
				return fmt.Errorf("unexpected response type: %T", resp)
			}

			// If no messages returned, we've reached the end
			if len(batchMessages) == 0 {
				break
			}

			// Append batch messages to all messages
			allMessages = append(allMessages, batchMessages...)

			// If we got less messages than the limit, we've reached the end
			if len(batchMessages) < limit {
				break
			}

			// Increment offset for next batch
			offset += limit
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Failed to run Telegram client: %v", err)
	}

	return allMessages, nil
}

func TgUserClientKeepSessionAlive() error {
	// Get config from environment
	config, err := NewTelegramUserClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get telegram config: %w", err)
	}

	client := telegram.NewClient(config.appId, config.appHash, telegram.Options{SessionStorage: storage})

	err = client.Run(context.Background(), func(ctx context.Context) error {
		// Ensure we're authorized
		if err := ensureAuthorized(ctx, client, config.phoneNumber, config.password); err != nil {
			return err
		}

		// Make a simple request to keep session alive
		api := client.API()
		_, err := api.HelpGetConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get self user info: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to keep session alive: %w", err)
	}

	return nil
}

func SetVerificationCode(code string) {
	codeMutex.Lock()
	defer codeMutex.Unlock()
	verificationCode = code
}

func getPeerInfoByChatId(chatId int64, tgClient *telegram.Client, ctx context.Context) (tg.InputPeerClass, error) {
	var inputPeer tg.InputPeerClass

	api := tgClient.API()

	// Fetch dialogs to find the chat and get its AccessHash if needed
	dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		Limit:      100, // Adjust the limit as needed
		OffsetPeer: &tg.InputPeerChat{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dialogs: %v", err)
	}

	found := false

	switch dlg := dialogs.(type) {
	case *tg.MessagesDialogsSlice:
		for _, chat := range dlg.Chats {
			switch c := chat.(type) {
			case *tg.Chat:
				if c.ID == chatId {
					inputPeer = &tg.InputPeerChat{
						ChatID: c.ID,
					}
					found = true
					break
				}
			case *tg.Channel:
				if c.ID == chatId {
					inputPeer = &tg.InputPeerChannel{
						ChannelID:  c.ID,
						AccessHash: c.AccessHash,
					}
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	default:
		return nil, fmt.Errorf("unexpected response type: %T", dialogs)
	}

	if !found {
		return nil, fmt.Errorf("chat with ID %d not found", chatId)
	}

	return inputPeer, nil
}

func ensureAuthorized(ctx context.Context, client *telegram.Client, phoneNumber, password string) error {
	authCli := client.Auth()

	status, err := authCli.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth status: %w", err)
	}

	if !status.Authorized {
		code, err := authCli.SendCode(ctx, phoneNumber, auth.SendCodeOptions{
			AllowAppHash: true,
		})
		if err != nil {
			return fmt.Errorf("failed to send code: %w", err)
		}
		sentCode := code.(*tg.AuthSentCode)

		// Replace environment variable with in-memory code
		codeMutex.RLock()
		receivedCode := verificationCode
		codeMutex.RUnlock()

		if receivedCode == "" {
			return fmt.Errorf("verification code not set - use /code command to set it")
		} else {
			log.Print("Code applied")
		}

		_, err = authCli.SignIn(ctx, phoneNumber, receivedCode, sentCode.PhoneCodeHash)
		if err != nil {
			if strings.Contains(err.Error(), "2FA required") {
				_, err = authCli.Password(ctx, password)
				if err != nil {
					return fmt.Errorf("failed to send 2FA password: %w", err)
				}
			} else {
				return fmt.Errorf("SignIn error: %w", err)
			}
		}
	}

	return err
}
