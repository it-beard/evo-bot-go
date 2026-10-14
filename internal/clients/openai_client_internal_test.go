package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServerClient(t *testing.T, handler http.HandlerFunc) *OpenAiClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewOpenAiClientWithOptions(
		option.WithAPIKey("test"),
		option.WithBaseURL(server.URL),
		option.WithMaxRetries(0),
	)
}

func TestOpenAiClient_RequestWithoutDeadlineTimesOut(t *testing.T) {
	previous := completionTimeout
	completionTimeout = 50 * time.Millisecond
	t.Cleanup(func() { completionTimeout = previous })

	// The server answers far later than the timeout; cleanups run in reverse order,
	// so the handler is released before the server waits for its connections
	release := make(chan struct{})
	client := newTestServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	})
	t.Cleanup(func() { close(release) })

	start := time.Now()
	_, err := client.GetCompletionWithReasoning(context.Background(), "prompt", openai.ReasoningEffortMinimal)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second, "a hanging OpenAI request must not block the handler forever")
}

func TestOpenAiClient_ResponseWithoutChoicesIsAnError(t *testing.T) {
	client := newTestServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"gpt-5-mini","choices":[]}`))
	})

	result, err := client.GetCompletion(context.Background(), "prompt")

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "no choices")
}
