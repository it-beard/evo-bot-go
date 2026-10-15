package clients

import (
	"context"
	"fmt"
	"time"

	"evo-bot-go/internal/config"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// completionTimeout limits a completion request whose context has no deadline
var completionTimeout = 5 * time.Minute

type OpenAiClient struct {
	client *openai.Client
}

func NewOpenAiClient() (*OpenAiClient, error) {
	// Load configuration
	appConfig, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	apiKey := appConfig.OpenAIAPIKey

	return NewOpenAiClientWithOptions(option.WithAPIKey(apiKey)), nil
}

// NewOpenAiClientWithOptions creates a client with explicit request options (e.g. a custom base URL in tests)
func NewOpenAiClientWithOptions(opts ...option.RequestOption) *OpenAiClient {
	client := openai.NewClient(opts...)

	return &OpenAiClient{
		client: &client,
	}
}

// GetCompletion sends a message to OpenAI and returns the response
func (c *OpenAiClient) GetCompletion(ctx context.Context, message string) (string, error) {
	return c.GetCompletionWithReasoning(ctx, message, openai.ReasoningEffortMedium)
}

// GetCompletionWithReasoning sends a message to OpenAI with specified reasoning effort and returns the response
func (c *OpenAiClient) GetCompletionWithReasoning(ctx context.Context, message string, reasoningEffort openai.ReasoningEffort) (string, error) {
	// Never wait forever: callers that pass a context without deadline get the default timeout
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, completionTimeout)
		defer cancel()
	}

	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(message),
		},
		Model:           openai.ChatModelGPT5Mini,
		ReasoningEffort: reasoningEffort,
		//Model: openai.ChatModelO3Mini,
		//Model: "o4-mini",
		//Model: "gpt-4.1-mini",
	})
	if err != nil {
		return "", fmt.Errorf("failed to get completion: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("failed to get completion: the response has no choices")
	}

	return completion.Choices[0].Message.Content, nil
}

// GetEmbedding generates an embedding vector for the given text using the text-embedding-ada-002 model
func (c *OpenAiClient) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	embedding, err := c.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: []string{text},
		},
		Model: openai.EmbeddingModelTextEmbeddingAda002,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	if len(embedding.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return embedding.Data[0].Embedding, nil
}

// GetBatchEmbeddings generates embedding vectors for multiple texts in a single API call
func (c *OpenAiClient) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return [][]float64{}, nil
	}

	embedding, err := c.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model: openai.EmbeddingModelTextEmbeddingAda002,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get batch embeddings: %w", err)
	}

	if len(embedding.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	result := make([][]float64, len(embedding.Data))
	for i, data := range embedding.Data {
		result[i] = data.Embedding
	}

	return result, nil
}
