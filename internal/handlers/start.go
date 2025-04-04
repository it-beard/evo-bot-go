package handlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
)

const (
	// Conversation states names
	startHandlerStateProcessCallback = "start_handler_state_process_callback"
	// Callbacks names
	startHandlerCallbackHelp = "start_handler_callback_help"
)

type startHandler struct {
	config               *config.Config
	messageSenderService services.MessageSenderService
}

func NewStartHandler(config *config.Config, messageSenderService services.MessageSenderService) ext.Handler {
	h := &startHandler{
		config:               config,
		messageSenderService: messageSenderService,
	}
	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.StartCommand, h.handleStart),
		},
		map[string][]ext.Handler{
			startHandlerStateProcessCallback: {
				handlers.NewCallback(callbackquery.Equal(startHandlerCallbackHelp), h.handleCallbackHelp),
			},
		},
		nil,
	)
}

func (h *startHandler) handleStart(b *gotgbot.Bot, ctx *ext.Context) error {
	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	userName := ""
	if ctx.EffectiveUser.FirstName != "" {
		userName = ctx.EffectiveUser.FirstName
	}

	greeting := "Приветствую"
	if userName != "" {
		greeting += ", *" + userName + "*"
	}
	greeting += "! 🎩"

	// Check if user is a member of the club
	isClubMember := utils.IsUserClubMember(b, ctx.EffectiveMessage, h.config)

	var message string
	var inlineKeyboard gotgbot.InlineKeyboardMarkup
	if isClubMember {
		// Message for club members
		message = greeting + "\n\n" +
			"Я — *Дженкинс Вебстер*, потомственный дворецкий и верный помощник клуба _\"Эволюция Кода\"_ 🧐\n\n" +
			"Рад видеть тебя среди участников нашего клуба! Я готов помочь тебе во всех твоих начинаниях. 🤵"

		inlineKeyboard = gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         "💡 Как пользоваться ботом?",
						CallbackData: startHandlerCallbackHelp,
					},
				},
			},
		}
	} else {
		// Message for non-members
		message = greeting + "\n\n" +
			"Я — *Дженкинс Вебстер*, потомственный дворецкий и верный помощник клуба _\"Эволюция Кода\"_ 🧐\n\n" +
			"Позвольте предложить тебе присоединиться к нашему изысканному сообществу разработчиков и разработчиц, " +
			"где я буду рад служить тебе всеми своими возможностями и ресурсами."

		inlineKeyboard = gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text: "💡 Жду тебя в клубе!",
						Url:  "https://web.tribute.tg/l/ge",
					},
				},
			},
		}
	}

	h.messageSenderService.ReplyMarkdown(
		b,
		ctx.EffectiveMessage,
		message,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: inlineKeyboard,
		},
	)

	return handlers.NextConversationState(startHandlerStateProcessCallback)
}

func (h *startHandler) handleCallbackHelp(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	isAdmin := utils.IsUserAdminOrCreator(b, ctx.EffectiveMessage.From.Id, h.config.SuperGroupChatID)
	helpText := utils.FormatHelpMessage(isAdmin)

	h.messageSenderService.ReplyHtml(b, ctx.EffectiveMessage, helpText, nil)

	return handlers.EndConversation()
}
