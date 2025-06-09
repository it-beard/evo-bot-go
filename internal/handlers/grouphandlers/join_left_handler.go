package grouphandlers

import (
	"evo-bot-go/internal/database/repositories"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/chatmember"
)

type JoinLeftHandler struct {
	userRepo *repositories.UserRepository
}

func NewJoinLeftHandler(userRepo *repositories.UserRepository) ext.Handler {
	h := &JoinLeftHandler{userRepo: userRepo}
	return handlers.NewChatMember(chatmember.All, h.handle)
}

func (h *JoinLeftHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	chatMember := ctx.ChatMember
	user := chatMember.NewChatMember.GetUser()

	newStatus := chatMember.NewChatMember.GetStatus()

	isNowMember := newStatus == "member" || newStatus == "administrator" || newStatus == "creator"
	isNowLeftOrBanned := newStatus == "left" || newStatus == "kicked"

	if isNowMember {
		dbUser, _, err := h.userRepo.GetOrFullCreate(&user)
		if err != nil {
			return fmt.Errorf("JoinLeftHandler: failed to get or create user in JoinLeftHandler: %w", err)
		}
		log.Printf("JoinLeftHandler: User %s (%d) is now a member, setting IsClubMember to true", user.Username, user.Id)
		err = h.userRepo.SetClubMemberStatus(dbUser.ID, true)
		if err != nil {
			return fmt.Errorf("JoinLeftHandler: failed to set club member status to true for user %d: %w", dbUser.ID, err)
		}

	} else if isNowLeftOrBanned {
		dbUser, _, err := h.userRepo.GetOrFullCreate(&user)
		if err != nil {
			log.Printf("JoinLeftHandler: User %d who is now left/banned not found in DB, nothing to update: %v", user.Id, err)
			return nil // Not an error, user might have never interacted with the bot
		}

		if dbUser.IsClubMember {
			log.Printf("JoinLeftHandler: User %s (%d) is now left/banned, setting IsClubMember to false", user.Username, user.Id)
			err := h.userRepo.SetClubMemberStatus(dbUser.ID, false)
			if err != nil {
				return fmt.Errorf("JoinLeftHandler: failed to set club member status to false for user %d: %w", dbUser.ID, err)
			}
		}
	}

	return nil
}
