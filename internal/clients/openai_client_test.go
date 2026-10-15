package clients_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/testutils"

	"github.com/openai/openai-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setRequiredConfigEnv sets the minimal environment required by config.LoadConfig.
func setRequiredConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TG_EVO_BOT_TOKEN", "test-token")
	t.Setenv("TG_EVO_BOT_SUPERGROUP_CHAT_ID", "123")
	t.Setenv("TG_EVO_BOT_OPENAI_API_KEY", "sk-from-config")
	t.Setenv("TG_EVO_BOT_DB_CONNECTION", "postgres://localhost/db")
	t.Setenv("TG_EVO_BOT_MONITORED_TOPICS_IDS", "1")
	t.Setenv("TG_EVO_BOT_SUMMARY_TOPIC_ID", "2")
	t.Setenv("TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID", "3")
}

func TestNewOpenAiClient_UsesApiKeyFromConfig(t *testing.T) {
	setRequiredConfigEnv(t)

	var (
		mu         sync.Mutex
		authHeader string
		path       string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authHeader = r.Header.Get("Authorization")
		path = r.URL.Path
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.5]}],"model":"text-embedding-ada-002"}`))
	}))
	t.Cleanup(server.Close)
	// openai.NewClient reads OPENAI_BASE_URL from the environment by default
	t.Setenv("OPENAI_BASE_URL", server.URL)

	client, err := clients.NewOpenAiClient()
	require.NoError(t, err)
	require.NotNil(t, client)

	vector, err := client.GetEmbedding(context.Background(), "hello")

	require.NoError(t, err)
	assert.Equal(t, []float64{0.5}, vector)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "Bearer sk-from-config", authHeader)
	assert.Equal(t, "/embeddings", path)
}

func TestNewOpenAiClient_ConfigError(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("TG_EVO_BOT_TOKEN", "")

	client, err := clients.NewOpenAiClient()

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to load configuration")
	assert.Contains(t, err.Error(), "TG_EVO_BOT_TOKEN")
}

func TestOpenAiClient_GetCompletion_Success(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.SetCompletion("answer text")

	result, err := client.GetCompletion(context.Background(), "what is go?")

	require.NoError(t, err)
	assert.Equal(t, "answer text", result)
	assert.Equal(t, "what is go?", mock.LastPrompt())

	requests := mock.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "/chat/completions", requests[0].Path)
	assert.Equal(t, string(openai.ChatModelGPT5Mini), requests[0].Body["model"])
	assert.Equal(t, "medium", requests[0].Body["reasoning_effort"])

	messages, ok := requests[0].Body["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	first, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", first["role"])
}

func TestOpenAiClient_GetCompletionWithReasoning_PassesReasoningEffort(t *testing.T) {
	efforts := []openai.ReasoningEffort{
		openai.ReasoningEffortLow,
		openai.ReasoningEffortMedium,
		openai.ReasoningEffortHigh,
	}

	for _, effort := range efforts {
		t.Run(string(effort), func(t *testing.T) {
			client, mock := testutils.NewMockOpenAI(t)
			mock.SetCompletion("reasoned")

			result, err := client.GetCompletionWithReasoning(context.Background(), "prompt", effort)

			require.NoError(t, err)
			assert.Equal(t, "reasoned", result)
			requests := mock.Requests()
			require.Len(t, requests, 1)
			assert.Equal(t, string(effort), requests[0].Body["reasoning_effort"])
			assert.Equal(t, "prompt", mock.LastPrompt())
		})
	}
}

func TestOpenAiClient_GetCompletion_HTTPError(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.FailWith(http.StatusInternalServerError)

	result, err := client.GetCompletion(context.Background(), "prompt")

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "failed to get completion")
	assert.Len(t, mock.Requests(), 1, "client must not retry")
}

func TestOpenAiClient_GetCompletion_CanceledContext(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := client.GetCompletion(ctx, "prompt")

	require.Error(t, err)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, mock.Requests())
}

func TestOpenAiClient_GetEmbedding_Success(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.SetEmbeddings([][]float64{{0.25, -0.5, 1}})

	vector, err := client.GetEmbedding(context.Background(), "some text")

	require.NoError(t, err)
	assert.Equal(t, []float64{0.25, -0.5, 1}, vector)

	requests := mock.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "/embeddings", requests[0].Path)
	assert.Equal(t, string(openai.EmbeddingModelTextEmbeddingAda002), requests[0].Body["model"])
	assert.Equal(t, []any{"some text"}, requests[0].Body["input"])
}

func TestOpenAiClient_GetEmbedding_ReturnsFirstVector(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.SetEmbeddings([][]float64{{1}, {2}})

	vector, err := client.GetEmbedding(context.Background(), "text")

	require.NoError(t, err)
	assert.Equal(t, []float64{1}, vector)
}

func TestOpenAiClient_GetEmbedding_EmptyData(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.SetEmbeddings([][]float64{})

	vector, err := client.GetEmbedding(context.Background(), "text")

	require.Error(t, err)
	assert.Nil(t, vector)
	assert.EqualError(t, err, "no embedding data returned")
}

func TestOpenAiClient_GetEmbedding_HTTPError(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.FailWith(http.StatusBadRequest)

	vector, err := client.GetEmbedding(context.Background(), "text")

	require.Error(t, err)
	assert.Nil(t, vector)
	assert.Contains(t, err.Error(), "failed to get embedding")
}

func TestOpenAiClient_GetBatchEmbeddings_Success(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.SetEmbeddings([][]float64{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}})

	vectors, err := client.GetBatchEmbeddings(context.Background(), []string{"a", "b", "c"})

	require.NoError(t, err)
	assert.Equal(t, [][]float64{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}}, vectors)

	requests := mock.Requests()
	require.Len(t, requests, 1, "batch must be sent in a single request")
	assert.Equal(t, "/embeddings", requests[0].Path)
	assert.Equal(t, []any{"a", "b", "c"}, requests[0].Body["input"])
	assert.Equal(t, string(openai.EmbeddingModelTextEmbeddingAda002), requests[0].Body["model"])
}

func TestOpenAiClient_GetBatchEmbeddings_EmptyInput(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
	}{
		{name: "nil slice", texts: nil},
		{name: "empty slice", texts: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := testutils.NewMockOpenAI(t)

			vectors, err := client.GetBatchEmbeddings(context.Background(), tt.texts)

			require.NoError(t, err)
			require.NotNil(t, vectors)
			assert.Empty(t, vectors)
			assert.Empty(t, mock.Requests(), "no API call expected for empty input")
		})
	}
}

func TestOpenAiClient_GetBatchEmbeddings_EmptyData(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.SetEmbeddings([][]float64{})

	vectors, err := client.GetBatchEmbeddings(context.Background(), []string{"a"})

	require.Error(t, err)
	assert.Nil(t, vectors)
	assert.EqualError(t, err, "no embedding data returned")
}

func TestOpenAiClient_GetBatchEmbeddings_HTTPError(t *testing.T) {
	client, mock := testutils.NewMockOpenAI(t)
	mock.FailWith(http.StatusServiceUnavailable)

	vectors, err := client.GetBatchEmbeddings(context.Background(), []string{"a", "b"})

	require.Error(t, err)
	assert.Nil(t, vectors)
	assert.Contains(t, err.Error(), "failed to get batch embeddings")
}

func TestOpenAiClient_GetBatchEmbeddings_PreservesResponseOrder(t *testing.T) {
	// The client maps response data positionally; verify it with a custom server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Input []string `json:"input"`
		}
		_ = json.Unmarshal(raw, &body)

		data := make([]map[string]any, len(body.Input))
		for i := range body.Input {
			data[i] = map[string]any{"object": "embedding", "index": i, "embedding": []float64{float64(i)}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data, "model": "text-embedding-ada-002"})
	}))
	t.Cleanup(server.Close)

	t.Setenv("OPENAI_BASE_URL", server.URL)
	setRequiredConfigEnv(t)
	client, err := clients.NewOpenAiClient()
	require.NoError(t, err)

	vectors, err := client.GetBatchEmbeddings(context.Background(), []string{"x", "y", "z", "w"})

	require.NoError(t, err)
	assert.Equal(t, [][]float64{{0}, {1}, {2}, {3}}, vectors)
}
