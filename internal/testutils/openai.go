package testutils

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"evo-bot-go/internal/clients"

	"github.com/openai/openai-go/v2/option"
)

// OpenAIRequest is a request received by the fake OpenAI server
type OpenAIRequest struct {
	Path string
	Body map[string]any
}

// MockOpenAI is a fake OpenAI HTTP API for chat completions and embeddings
type MockOpenAI struct {
	mu             sync.Mutex
	requests       []OpenAIRequest
	completion     string
	embeddings     [][]float64
	failStatusCode int
}

// NewMockOpenAI starts a fake OpenAI server and returns a client pointed at it (no retries)
func NewMockOpenAI(t *testing.T) (*clients.OpenAiClient, *MockOpenAI) {
	t.Helper()

	mock := &MockOpenAI{
		completion: "mock completion",
		embeddings: [][]float64{{0.1, 0.2, 0.3}},
	}
	server := httptest.NewServer(http.HandlerFunc(mock.serve))
	t.Cleanup(server.Close)

	client := clients.NewOpenAiClientWithOptions(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
		option.WithMaxRetries(0),
	)
	return client, mock
}

// SetCompletion sets the content returned by chat completions
func (m *MockOpenAI) SetCompletion(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completion = content
}

// SetEmbeddings sets the vectors returned by the embeddings endpoint
func (m *MockOpenAI) SetEmbeddings(vectors [][]float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embeddings = vectors
}

// FailWith makes every request fail with the given HTTP status (0 disables failures)
func (m *MockOpenAI) FailWith(statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failStatusCode = statusCode
}

// Requests returns all received requests
func (m *MockOpenAI) Requests() []OpenAIRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]OpenAIRequest(nil), m.requests...)
}

// LastPrompt returns the content of the first message of the last chat completion request
func (m *MockOpenAI) LastPrompt() string {
	reqs := m.Requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		messages, ok := reqs[i].Body["messages"].([]any)
		if !ok || len(messages) == 0 {
			continue
		}
		if first, ok := messages[0].(map[string]any); ok {
			if content, ok := first["content"].(string); ok {
				return content
			}
		}
	}
	return ""
}

func (m *MockOpenAI) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)

	m.mu.Lock()
	m.requests = append(m.requests, OpenAIRequest{Path: r.URL.Path, Body: body})
	completion, embeddings, failStatusCode := m.completion, m.embeddings, m.failStatusCode
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if failStatusCode != 0 {
		w.WriteHeader(failStatusCode)
		_, _ = w.Write([]byte(`{"error":{"message":"mock failure","type":"server_error"}}`))
		return
	}

	switch r.URL.Path {
	case "/chat/completions":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   "gpt-5-mini",
			"choices": []any{map[string]any{
				"index":         0,
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": completion},
			}},
		})
	case "/embeddings":
		data := make([]any, len(embeddings))
		for i, vec := range embeddings {
			data[i] = map[string]any{"object": "embedding", "index": i, "embedding": vec}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   data,
			"model":  "text-embedding-ada-002",
			"usage":  map[string]any{"prompt_tokens": 1, "total_tokens": 1},
		})
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"unknown path"}}`))
	}
}
