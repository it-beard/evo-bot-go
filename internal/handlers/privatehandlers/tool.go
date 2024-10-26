package privatehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"your_module_name/internal/clients"
	"your_module_name/internal/handlers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const toolHandlerName = "tool_handler"
const toolCommand = "/tool"
const privateChat = "private"

type ToolHandler struct {
	openaiClient *clients.OpenAiClient
	chatId       int64 // Add field for chat ID
	topicId      int   // Add field for topic ID
}

func NewToolHandler(openaiClient *clients.OpenAiClient) handlers.Handler {
	// Parse environment variables
	chatIdStr := os.Getenv("TG_EVO_BOT_MAIN_CHAT_ID")
	topicIdStr := os.Getenv("TG_EVO_BOT_TOOL_TOPIC_ID")

	chatId, err := strconv.ParseInt(chatIdStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TG_EVO_BOT_TOOL_CHAT_ID: %v", err)
	}

	topicId, err := strconv.Atoi(topicIdStr)
	if err != nil {
		log.Fatalf("Invalid TG_EVO_BOT_TOOL_TOPIC_ID: %v", err)
	}

	return &ToolHandler{
		openaiClient: openaiClient,
		chatId:       chatId,
		topicId:      topicId,
	}
}

func (h *ToolHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract text after command
	commandText := strings.TrimPrefix(msg.Text, toolCommand)
	commandText = strings.TrimSpace(commandText)
	if commandText == "" {
		_, err := msg.Reply(b, fmt.Sprintf("Пожалуйста, введи текст после команды. Пример: %s <текст>", toolCommand), nil)
		return err
	}

	// Get messages from chat
	messages, err := clients.GetChatMessagesNew(h.chatId, h.topicId, 100) // Get last 100 messages
	if err != nil {
		return fmt.Errorf("failed to get chat messages: %w", err)
	}

	type MessageObject struct {
		ID      int    `json:"id"`
		Message string `json:"message"`
	}

	messageObjects := make([]MessageObject, 0, len(messages))
	for _, message := range messages {
		messageObjects = append(messageObjects, MessageObject{
			ID:      message.ID, // assuming Message struct has ID field
			Message: message.Message,
		})
	}

	db, err := json.Marshal(messageObjects)
	if err != nil {
		return fmt.Errorf("failed to marshal messages to JSON: %w", err)
	}
	if len(messageObjects) == 0 {
		return fmt.Errorf("no messages found in chat")
	}

	if string(db) == "" {
		return fmt.Errorf("no messages found in chat")
	}

	// Create prompt for OpenAI
	prompt := "Ты помощник по поиску информации об ИИ-инструментах для разработки. \\n" +
		"Вся информация об инструментах содержиться в формате json в БАЗЕ ДАННЫХ ниже. \\n" +
		"Нужно найти самые релевантные инструменты по запросу ПОЛЬЗОВАТЕЛЯ. \\n" +
		fmt.Sprintf("БАЗА ДАННЫХ инструментов: <database>%s</database>\\n", db) +
		fmt.Sprintf("Запрос ПОЛЬЗОВАТЕЛЯ: <request>%s</request>\\n", commandText) +
		"Отвечай только на русском языке. \\n" +
		"Саморизируй описание инструмента до двух предложений либо меньше. \\n" +
		"Если инструмента нет в базе данных, то ответь, что его нет. \\n" +
		"Если найдено полное совпадения инструмента по названию (пользователь указал конкретное название), то выведи только его и без нумерации. \\n" +
		"Если инструментов несколько, то расскажи про каждый из них отдельным блоком, но не более пяти инструментов в ответе. \\n" +
		"Самые релевантные инструменты должны идти первыми в ответе. \\n" +
		"Не используй в ответе хештеги, но обращай на них внимание при поиске. \\n" +
		"Для ответа используй форматирование Markdown. \\n" +
		"Оборачивай название инструмента ссылкой. \\n" +
		fmt.Sprintf("Вид ссылки: https://t.me/c/%d/%d/ID, где ID - это ID инструмента в базе данных. \\n", h.chatId, h.topicId) +
		"Пример ответа для одного инструмента: \\n" +
		"Инструмент 1 - описание инструмента 1 \\n" +
		"Пример ответа для нескольких инструментов: \\n" +
		"1. Инструмент 1 - описание инструмента 1 \\n" +
		"2. Инструмент 2 - описание инструмента 2 \\n" +
		"3. Инструмент 3 - описание инструмента 3 \\n" +
		fmt.Sprintf("Если по твоему мнению в базе есть больше пяти релевантных инструментов, то предложи ознакомиться с дополнительными инструментами непосредственно в чате \"Инструменты\". (слово \"Инструменты\" оборачивай в markdown-ссылку https://t.me/c/%d/%d)\\n", h.chatId, h.topicId)

	// Get completion from OpenAI
	responseOpenAi, err := h.openaiClient.GetCompletion(context.TODO(), prompt)
	if err != nil {
		return fmt.Errorf("failed to get OpenAI completion: %w", err)
	}

	_, err = msg.Reply(b, responseOpenAi, &gotgbot.SendMessageOpts{
		ParseMode: "Markdown",
	})
	return err
}

func (h *ToolHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	chatId, err := strconv.ParseInt("-100"+strconv.FormatInt(h.chatId, 10), 10, 64)
	if err != nil {
		log.Printf("Failed to parse chat ID: %v", err)
		return false
	}
	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, msg.From.Id, nil)
	if err != nil {
		log.Printf("Failed to get chat member: %v", err)
		return false
	}

	status := chatMember.GetStatus()
	if status == "left" || status == "kicked" {
		msg.Reply(b, "Команда доступна только для членов клуба.", nil)
		log.Print("Trying to use /tool command without club membership")
		return false
	}
	return msg.Text != "" && strings.HasPrefix(msg.Text, toolCommand) && msg.Chat.Type == privateChat
}

func (h *ToolHandler) Name() string {
	return toolHandlerName
}
