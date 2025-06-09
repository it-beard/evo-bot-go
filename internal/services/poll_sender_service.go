package services

import (
	"evo-bot-go/internal/utils"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// PollSenderService handles sending polls to Telegram
type PollSenderService struct {
	bot *gotgbot.Bot
}

// NewPollSenderService creates a new poll sender service
func NewPollSenderService(bot *gotgbot.Bot) *PollSenderService {
	return &PollSenderService{
		bot: bot,
	}
}

// SendPoll sends a poll to the specified chat
func (s *PollSenderService) SendPoll(
	chatID int64,
	question string,
	answers []gotgbot.InputPollOption,
	options *gotgbot.SendPollOpts,
) (*gotgbot.Message, error) {
	log.Printf("%s: Sending poll to chat ID %d", utils.GetCurrentTypeName(), chatID)

	if options != nil && options.MessageThreadId != 0 {
		log.Printf("%s: Poll will be sent to topic ID %d", utils.GetCurrentTypeName(), options.MessageThreadId)
	}

	sentPollMsg, err := s.bot.SendPoll(
		chatID,
		question,
		answers,
		options,
	)
	if err != nil {
		log.Printf("%s: Failed to send poll: %v", utils.GetCurrentTypeName(), err)
		return nil, err
	}

	log.Printf(
		"%s: Poll sent successfully. MessageID: %d, ChatID: %d",
		utils.GetCurrentTypeName(),
		sentPollMsg.MessageId,
		sentPollMsg.Chat.Id,
	)
	return sentPollMsg, nil
}

// StopPoll stops a poll in the specified chat
func (s *PollSenderService) StopPoll(
	chatID int64,
	messageID int64,
	options *gotgbot.StopPollOpts,
) (*gotgbot.Poll, error) {
	log.Printf("%s: Stopping poll in chat ID %d, message ID %d", utils.GetCurrentTypeName(), chatID, messageID)

	stoppedPoll, err := s.bot.StopPoll(chatID, messageID, options)
	if err != nil {
		log.Printf("%s: Failed to stop poll: %v", utils.GetCurrentTypeName(), err)
		return nil, err
	}

	log.Printf("%s: Poll stopped successfully. Poll ID: %s", utils.GetCurrentTypeName(), stoppedPoll.Id)
	return stoppedPoll, nil
}
