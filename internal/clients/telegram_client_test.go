package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "123456:SECRET-token_value"

// fakeBotClient returns a fixed result for every request
type fakeBotClient struct {
	gotgbot.BotClient
	result json.RawMessage
	err    error
}

func (f *fakeBotClient) RequestWithContext(context.Context, string, string, map[string]string, map[string]gotgbot.FileReader, *gotgbot.RequestOpts) (json.RawMessage, error) {
	return f.result, f.err
}

func request(t *testing.T, inner *fakeBotClient) (json.RawMessage, error) {
	t.Helper()
	client := &RedactingBotClient{BotClient: inner}
	return client.RequestWithContext(context.Background(), testToken, "sendMessage", nil, nil, nil)
}

func TestRedactingBotClient_TransportErrorHidesToken(t *testing.T) {
	// This is how gotgbot wraps a failed HTTP call: the *url.Error text contains the request URL
	urlErr := &url.Error{
		Op:  "Post",
		URL: "https://api.telegram.org/bot" + testToken + "/sendMessage",
		Err: errors.New("dial tcp: i/o timeout"),
	}
	inner := &fakeBotClient{err: fmt.Errorf("failed to execute POST request to sendMessage: %w", urlErr)}

	_, err := request(t, inner)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), testToken)
	assert.NotContains(t, err.Error(), "SECRET")
	assert.Contains(t, err.Error(), "https://api.telegram.org/bot<bot-token>/sendMessage")
	assert.Contains(t, err.Error(), "i/o timeout")

	var gotURLErr *url.Error
	require.True(t, errors.As(err, &gotURLErr), "the error chain is kept")
}

func TestRedactingBotClient_AnyErrorTextWithTokenIsRedacted(t *testing.T) {
	inner := &fakeBotClient{err: errors.New("something went wrong for " + testToken)}

	_, err := request(t, inner)

	require.Error(t, err)
	assert.Equal(t, "something went wrong for <bot-token>", err.Error())
}

func TestRedactingBotClient_TelegramErrorsAndResultsPassThrough(t *testing.T) {
	telegramErr := &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: "Bad Request: TOPIC_CLOSED"}

	_, err := request(t, &fakeBotClient{err: telegramErr})
	assert.Same(t, telegramErr, err)

	result, err := request(t, &fakeBotClient{result: json.RawMessage(`{"ok":true}`)})
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(result))
}

func TestNewRedactingBotClient_WrapsDefaultClient(t *testing.T) {
	client := NewRedactingBotClient()

	require.NotNil(t, client.BotClient)
	assert.IsType(t, &gotgbot.BaseBotClient{}, client.BotClient)
	assert.Equal(t, "https://api.telegram.org", client.GetAPIURL(nil))
}
