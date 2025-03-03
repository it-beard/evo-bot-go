package rag

import (
	"context"
	"sort"

	"your_module_name/internal/storage"
)

// Retriever handles retrieval of relevant messages
type Retriever struct {
	embeddingService *EmbeddingService
}

// NewRetriever creates a new retriever
func NewRetriever(embeddingService *EmbeddingService) *Retriever {
	return &Retriever{
		embeddingService: embeddingService,
	}
}

// ScoredMessage represents a message with a relevance score
type ScoredMessage struct {
	Message storage.Message
	Score   float32
}

// RetrieveRelevantMessages retrieves the most relevant messages for a given query
func (r *Retriever) RetrieveRelevantMessages(ctx context.Context, messages []storage.Message, query string, topK int) ([]ScoredMessage, error) {
	// Generate embeddings for all messages
	messagesWithEmbeddings, err := r.embeddingService.GenerateEmbeddings(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Generate embedding for the query
	queryEmbedding, err := r.embeddingService.generateEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}

	// Calculate similarity scores
	var scoredMessages []ScoredMessage
	for _, msgWithEmb := range messagesWithEmbeddings {
		similarity := CosineSimilarity(queryEmbedding, msgWithEmb.Embedding)
		scoredMessages = append(scoredMessages, ScoredMessage{
			Message: msgWithEmb.Message,
			Score:   similarity,
		})
	}

	// Sort by similarity score (descending)
	sort.Slice(scoredMessages, func(i, j int) bool {
		return scoredMessages[i].Score > scoredMessages[j].Score
	})

	// Return top K results
	if len(scoredMessages) > topK {
		scoredMessages = scoredMessages[:topK]
	}

	return scoredMessages, nil
}

// BuildContextFromMessages builds a context string from a list of messages
func BuildContextFromMessages(messages []ScoredMessage) string {
	context := "Here are the most relevant messages from the chat:\n\n"

	for i, msg := range messages {
		context += "Message " + string(rune('A'+i)) + ":\n"
		context += "From: " + msg.Message.Username + "\n"
		context += "Text: " + msg.Message.Text + "\n"
		context += "Time: " + msg.Message.CreatedAt.Format("2006-01-02 15:04:05") + "\n\n"
	}

	return context
}
