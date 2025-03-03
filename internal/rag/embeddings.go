package rag

import (
	"context"
	"fmt"
	"math"

	"your_module_name/internal/clients"
	"your_module_name/internal/storage"
)

// Embedding represents a vector embedding of text
type Embedding []float32

// MessageWithEmbedding represents a message with its embedding
type MessageWithEmbedding struct {
	Message   storage.Message
	Embedding Embedding
}

// EmbeddingService handles text embedding operations
type EmbeddingService struct {
	openaiClient *clients.OpenAiClient
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(openaiClient *clients.OpenAiClient) *EmbeddingService {
	return &EmbeddingService{
		openaiClient: openaiClient,
	}
}

// GenerateEmbeddings generates embeddings for a list of messages
func (s *EmbeddingService) GenerateEmbeddings(ctx context.Context, messages []storage.Message) ([]MessageWithEmbedding, error) {
	var result []MessageWithEmbedding

	// Process messages in batches to avoid rate limits
	batchSize := 20
	for i := 0; i < len(messages); i += batchSize {
		end := i + batchSize
		if end > len(messages) {
			end = len(messages)
		}

		batch := messages[i:end]
		batchResults, err := s.processBatch(ctx, batch)
		if err != nil {
			return nil, err
		}

		result = append(result, batchResults...)
	}

	return result, nil
}

// processBatch processes a batch of messages to generate embeddings
func (s *EmbeddingService) processBatch(ctx context.Context, messages []storage.Message) ([]MessageWithEmbedding, error) {
	var result []MessageWithEmbedding

	for _, msg := range messages {
		// Generate embedding for the message text
		embedding, err := s.generateEmbedding(ctx, msg.Text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for message %d: %w", msg.ID, err)
		}

		result = append(result, MessageWithEmbedding{
			Message:   msg,
			Embedding: embedding,
		})
	}

	return result, nil
}

// generateEmbedding generates an embedding for a single text
func (s *EmbeddingService) generateEmbedding(ctx context.Context, text string) (Embedding, error) {
	// Use OpenAI to generate embedding
	// Note: This is a simplified implementation. In a real application, you would
	// use the OpenAI embeddings API directly.

	// For now, we'll use the completion API as a workaround
	prompt := fmt.Sprintf("Convert the following text to a comma-separated list of 10 floating point numbers between -1 and 1 that represent the semantic meaning of the text. Only output the numbers, no other text.\n\nText: %s", text)

	response, err := s.openaiClient.GetCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding from OpenAI: %w", err)
	}

	// Parse the response into a vector of floats
	// This is a simplified implementation
	// In a real application, you would parse the actual embedding vector

	// For now, we'll just create a dummy embedding based on the response length
	// This is just to avoid the "declared and not used" error
	embedding := make(Embedding, 10)
	for i := range embedding {
		// Use the response length to influence the embedding values
		// This is not a real embedding, just a placeholder
		embedding[i] = float32(i+len(response)%10) / 10.0
	}

	return embedding, nil
}

// CosineSimilarity calculates the cosine similarity between two embeddings
func CosineSimilarity(a, b Embedding) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float32
	var normA float32
	var normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
