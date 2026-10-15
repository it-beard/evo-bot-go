package handlers

import (
	"html"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/formatters"
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
	messageSenderService *services.MessageSenderService
	permissionsService   *services.PermissionsService
}

func NewStartHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &startHandler{
		config:               config,
		messageSenderService: messageSenderService,
		permissionsService:   permissionsService,
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
	user := ctx.EffectiveUser
	msg := ctx.EffectiveMessage
	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	greeting := "Приветствую"
	if user.FirstName != "" {
		greeting += ", <b>" + html.EscapeString(user.FirstName) + "</b>"
	}
	greeting += "! 🎩"

	// Check if user is a member of the club
	isClubMember := utils.IsUserClubMember(b, user.Id, h.config)

	var message string
	var inlineKeyboard gotgbot.InlineKeyboardMarkup
	if isClubMember {
		// Message for club members
		message = greeting + "\n\n" +
			"Я — <b>Дженкинс Вебстер</b>, потомственный дворецкий и верный помощник клуба <i>«Эволюция Кода»</i> 🧐\n\n" +
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
			"Я — <b>Дженкинс Вебстер</b>, потомственный дворецкий и верный помощник клуба <i>«Эволюция Кода»</i> 🧐\n\n" +
			"Позволь предложить тебе присоединиться к нашему изысканному сообществу разработчиков и разработчиц, " +
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

	h.messageSenderService.ReplyHtml(
		msg,
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

	user := ctx.EffectiveUser
	isAdmin := utils.IsUserAdminOrCreator(b, user.Id, h.config)
	helpText := formatters.FormatHelpMessage(isAdmin, h.config)

	h.messageSenderService.ReplyHtml(ctx.EffectiveMessage, helpText, nil)

	return handlers.EndConversation()
}
