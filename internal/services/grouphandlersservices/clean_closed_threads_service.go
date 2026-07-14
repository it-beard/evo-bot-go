package grouphandlersservices

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type CleanClosedThreadsService struct {
	config               *config.Config
	closedTopics         map[int]bool
	messageSenderService *services.MessageSenderService
	groupTopicRepository *repositories.GroupTopicRepository
}

func NewCleanClosedThreadsService(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	groupTopicRepository *repositories.GroupTopicRepository,
) *CleanClosedThreadsService {
	// Create map of closed topics
	closedTopics := make(map[int]bool)
	for _, id := range config.ClosedTopicsIDs {
		closedTopics[id] = true
	}
	return &CleanClosedThreadsService{
		config:               config,
		messageSenderService: messageSenderService,
		closedTopics:         closedTopics,
		groupTopicRepository: groupTopicRepository,
	}
}

func (h *CleanClosedThreadsService) CleanClosedThreads(msg *gotgbot.Message, b *gotgbot.Bot) error {
	// Delete original message
	_, err := msg.Delete(b, nil)
	if err != nil {
		return fmt.Errorf(
			"%s: error >> failed to delete message: %w",
			utils.GetCurrentTypeName(),
			err)
	}

	// Prepare messages
	chatIdStr := strconv.FormatInt(msg.Chat.Id, 10)[4:]
	topicName := "Topic name"
	topic, err := h.groupTopicRepository.GetGroupTopicByTopicID(msg.MessageThreadId)
	if err != nil {
		log.Printf("%s: error >> failed to get thread name: %v", utils.GetCurrentTypeName(), err)
	} else {
		topicName = topic.Name
	}
	mainConversationTopicName := "Main conversation topic name"
	mainConversationTopic, err := h.groupTopicRepository.GetGroupTopicByTopicID(int64(h.config.ForwardingTopicID))
	if err != nil {
		log.Printf("%s: error >> failed to get main conversation topic name: %v", utils.GetCurrentTypeName(), err)
	} else {
		mainConversationTopicName = mainConversationTopic.Name
	}
	threadUrl := fmt.Sprintf("<a href=\"https://telegram.me/c/%s/%d\">\"%s\"</a>", chatIdStr, msg.MessageThreadId, topicName)
	messageText := fmt.Sprintf(
		"<b>Приношу свои извинения</b> 🧐\n\n"+
			"Твоё сообщение в канале %s было удалено, поскольку этот канал предназначен только для чтения. \n\n"+
			"Однако ты можешь присоединиться к обсуждению, используя функцию <b>Reply</b> (ответ) на интересующий тебя пост. "+
			"Твой ответ автоматически появится в чате \"<i>%s</i>\" 👌\n\n"+
			"⬇️ <i>Копия твоего сообщения</i> ⬇️",
		threadUrl,
		mainConversationTopicName,
	)

	// Send notification to user
	err = h.messageSenderService.SendHtml(msg.From.Id, messageText, nil)
	if err != nil {
		return fmt.Errorf(
			"%s: error >> failed to send notification message: %w",
			utils.GetCurrentTypeName(),
			err)
	}
	// Send copy of the original message to user
	_, err = h.messageSenderService.SendCopy(msg.From.Id, nil, msg.Text, msg.Entities, msg)
	if err != nil {
		return fmt.Errorf(
			"%s: error >> failed to send copy of the original message: %w",
			utils.GetCurrentTypeName(),
			err)
	}

	// Log the deletion
	log.Printf(
		"%s: Deleted message in topic %s\n"+
			"User ID: %d\n"+
			"Content: \"%s\"",
		utils.GetCurrentTypeName(),
		threadUrl,
		msg.From.Id,
		msg.Text,
	)

	return nil
}

func (h *CleanClosedThreadsService) IsTopicShouldBeCleaned(msg *gotgbot.Message, b *gotgbot.Bot) bool {
	// Do nothing if message is not in closed topics
	if !h.closedTopics[int(msg.MessageThreadId)] {
		return false
	}

	// Don't trigger if message is reply to another message in thread (this already handled by RepliesFromThreadsHandler)
	if h.closedTopics[int(msg.MessageThreadId)] &&
		msg.ReplyToMessage != nil &&
		msg.ReplyToMessage.MessageId != msg.MessageThreadId {
		return false
	}

	// Don't trigger if message from admin or creator
	if utils.IsUserAdminOrCreator(b, msg.From.Id, h.config) {
		return false
	}

	// Don't trigger if message from bot with name "GroupAnonymousBot"
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		return false
	}

	return true
}
