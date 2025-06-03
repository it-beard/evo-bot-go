package services

import (
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
	log.Printf("Poll Sender Service: Sending poll to chat ID %d", chatID)

	if options != nil && options.MessageThreadId != 0 {
		log.Printf("Poll Sender Service: Poll will be sent to topic ID %d", options.MessageThreadId)
	}

	sentPollMsg, err := s.bot.SendPoll(
		chatID,
		question,
		answers,
		options,
	)
	if err != nil {
		log.Printf("Poll Sender Service: Failed to send poll: %v", err)
		return nil, err
	}

	log.Printf(
		"Poll Sender Service: Poll sent successfully. MessageID: %d, ChatID: %d",
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
	log.Printf("Poll Sender Service: Stopping poll in chat ID %d, message ID %d", chatID, messageID)

	stoppedPoll, err := s.bot.StopPoll(chatID, messageID, options)
	if err != nil {
		log.Printf("Poll Sender Service: Failed to stop poll: %v", err)
		return nil, err
	}

	log.Printf("Poll Sender Service: Poll stopped successfully. Poll ID: %s", stoppedPoll.Id)
	return stoppedPoll, nil
}
