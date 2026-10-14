package clients

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const redactedToken = "<bot-token>"

// RedactingBotClient wraps a gotgbot.BotClient and removes the bot token from request errors.
// Transport errors are *url.Error values whose text contains the request URL, and the Bot API URL
// contains the token, so without this every network hiccup would print the token to the logs.
type RedactingBotClient struct {
	gotgbot.BotClient
}

// NewRedactingBotClient wraps the default gotgbot client
func NewRedactingBotClient() *RedactingBotClient {
	return &RedactingBotClient{BotClient: &gotgbot.BaseBotClient{}}
}

func (c *RedactingBotClient) RequestWithContext(
	ctx context.Context,
	token string,
	method string,
	params map[string]string,
	data map[string]gotgbot.FileReader,
	opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	result, err := c.BotClient.RequestWithContext(ctx, token, method, params, data, opts)
	return result, redactToken(err, token)
}

// redactToken hides token in err while keeping the error chain (e.g. *gotgbot.TelegramError) intact
// whenever possible
func redactToken(err error, token string) error {
	if err == nil || token == "" {
		return err
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		urlErr.URL = strings.ReplaceAll(urlErr.URL, token, redactedToken)
	}

	// Wrapping errors (fmt.Errorf) keep the text formatted before the URL was redacted
	if strings.Contains(err.Error(), token) {
		return &redactedError{msg: strings.ReplaceAll(err.Error(), token, redactedToken), err: err}
	}
	return err
}

// redactedError replaces the text of an error while keeping it unwrappable
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Unwrap() error { return e.err }
