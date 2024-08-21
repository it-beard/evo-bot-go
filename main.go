package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func main() {
	// Replace with your bot token
	token := os.Getenv("TG_EVO_BOT_TOKEN")
	if token == "" {
		panic("TOKEN environment variable is empty")
	}

	// Create bot
	b, err := gotgbot.NewBot(token, nil)
	if err != nil {
		panic("failed to create new bot: " + err.Error())
	}

	// Create updater and dispatcher.
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	updater := ext.NewUpdater(dispatcher, nil)

	// Handler for new chat members and left chat members
	dispatcher.AddHandler(handlers.NewMessage(func(msg *gotgbot.Message) bool {
		return msg.NewChatMembers != nil || msg.LeftChatMember != nil
	}, deleteJoinLeaveMessages))

	// Handler for replies mentioning only the bot
	dispatcher.AddHandler(handlers.NewMessage(func(msg *gotgbot.Message) bool {
		return msg.ReplyToMessage != nil && strings.TrimSpace(msg.Text) == "@"+b.Username
	}, forwardReplyToPrivate))

	// Start receiving updates.
	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,

			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil {
		panic("failed to start polling: " + err.Error())
	}
	log.Printf("%s has been started...\n", b.User.Username)

	// Idle, to keep updates coming in, and avoid bot stopping.
	updater.Idle()
}

func deleteJoinLeaveMessages(b *gotgbot.Bot, ctx *ext.Context) error {
	// Delete the message
	_, err := ctx.EffectiveMessage.Delete(b, nil)
	if err != nil {
		log.Printf("Error deleting message: %v", err)
	} else {
		if ctx.EffectiveMessage.NewChatMembers != nil {
			log.Printf("New user joined. User ID: %v", ctx.EffectiveMessage.NewChatMembers[0].Username)
		} else if ctx.EffectiveMessage.LeftChatMember != nil {
			log.Printf("User left. User ID: %v", ctx.EffectiveMessage.LeftChatMember.Username)
		}
	}
	return nil
}

func forwardReplyToPrivate(b *gotgbot.Bot, ctx *ext.Context) error {
	replyMsg := ctx.EffectiveMessage.ReplyToMessage
	if replyMsg == nil {
		return nil
	}

	// Delete the reply message before sending the copy
	_, err := ctx.EffectiveMessage.Delete(b, nil)
	if err != nil {
		log.Printf("Error deleting reply message: %v", err)
		return err
	} else {
		log.Printf("Reply message deleted")
	}

	// Send a copy of the original message to the user who replied in markdown
	bottomText := "ссылка на сообщение"
	lengthBottomText := utf8.RuneCountInString(bottomText)
	messageText := fmt.Sprintf("%s\n %s", replyMsg.Text, bottomText)

	if replyMsg.Photo == nil {
		_, err = b.SendMessage(
			ctx.EffectiveUser.Id,
			messageText,
			&gotgbot.SendMessageOpts{
				Entities: append(replyMsg.Entities, gotgbot.MessageEntity{
					Type:   "blockquote",
					Offset: int64(utf8.RuneCountInString(messageText) - lengthBottomText),
					Length: int64(lengthBottomText),
				}, gotgbot.MessageEntity{
					Type:   "text_link",
					Offset: int64(utf8.RuneCountInString(messageText) - lengthBottomText),
					Length: int64(lengthBottomText),
					Url:    fmt.Sprintf("https://t.me/c/%s/%d", strconv.FormatInt(replyMsg.Chat.Id, 10)[4:], replyMsg.MessageId),
				}),
			})
		if err != nil {
			log.Printf("Error sending copy of message to user: %v", err)
			return err
		} else {
			log.Printf("Message sent to user: %v", messageText)
		}
	}

	return nil
}
