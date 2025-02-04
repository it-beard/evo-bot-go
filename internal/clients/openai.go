package clients

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type OpenAiClient struct {
	client *openai.Client
}

func NewOpenAiClient() (*OpenAiClient, error) {
	apiKey := os.Getenv("TG_EVO_BOT_OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_OPENAI_API_KEY environment variable is not set")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &OpenAiClient{
		client: client,
	}, nil
}

// GetCompletion sends a message to OpenAI and returns the response
func (c *OpenAiClient) GetCompletion(ctx context.Context, message string) (string, error) {
	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(message),
		}),
		Model: openai.F(openai.ChatModelO3Mini),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get completion: %w", err)
	}

	return completion.Choices[0].Message.Content, nil
}
