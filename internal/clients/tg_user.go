package clients

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

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

func GetChatMessagesNew(chatId int64, topicId int, limit int) ([]tg.Message, error) {
	var messages []tg.Message

	// Get config from environment
	config, err := NewTelegramUserClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram config: %w", err)
	}

	sessionPath := "session_phone.json"
	storage := &session.FileStorage{Path: sessionPath}
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

		// Get messages from chat
		resp, err := api.MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{
			Peer:  inputPeer,
			Limit: limit,
			MsgID: topicId,
		})
		if err != nil {
			log.Fatalf("failed to get messages: %v", err)
		}

		// Extract messages from response
		switch m := resp.(type) {
		case *tg.MessagesChannelMessages:
			for _, msg := range m.Messages {
				if message, ok := msg.(*tg.Message); ok {
					messages = append(messages, *message)
				}
			}
		default:
			return fmt.Errorf("unexpected response type: %T", resp)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Failed to run Telegram client: %v", err)
	}

	return messages, nil
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

		var receivedCode string
		fmt.Print("Enter the code you received: ")
		fmt.Scanln(&receivedCode)

		if receivedCode == "" {
			return fmt.Errorf("verification code cannot be empty")
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

	return nil
}
