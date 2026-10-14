package formatters

import (
	"strings"
	"testing"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"

	"github.com/stretchr/testify/assert"
)

const adminSectionHeader = "<b>🔐 Команды администратора</b>"
const testCommandsHeader = "<b>⚙️ Команды для тестирования</b>"

func helpTestConfig() *config.Config {
	return &config.Config{SuperGroupChatID: 1234567890, RandomCoffeeTopicID: 77}
}

func TestFormatHelpMessage_RegularUser(t *testing.T) {
	result := FormatHelpMessage(false, helpTestConfig())

	assert.True(t, strings.HasPrefix(result, "<b>📋 Функционал бота</b>\n\n"))

	for _, command := range []string{
		"/start", "/help", "/cancel", "/profile", "/tools", "/content", "/intro", "/events", "/topics", "/topicAdd",
	} {
		assert.Contains(t, result, "└ "+command+" - ", "help must describe %s", command)
	}

	assert.Contains(t, result, "<b>☕️ Random Coffee</b>")
	assert.Contains(t, result, `<a href="https://telegram.me/c/1234567890/77">«Random Coffee»</a>`)
	assert.True(t, strings.HasSuffix(result,
		"\n\n<i>💡 <a href=\"https://telegram.me/c/2069889012/127/9470\">Открыть полное руководство</a></i>"),
		"guide link must close the non-admin help")

	assert.NotContains(t, result, adminSectionHeader)
	assert.NotContains(t, result, testCommandsHeader)
	assert.NotContains(t, result, "/"+constants.EventSetupCommand)
}

func TestFormatHelpMessage_Admin(t *testing.T) {
	cfg := helpTestConfig()
	regular := FormatHelpMessage(false, cfg)

	result := FormatHelpMessage(true, cfg)

	assert.True(t, strings.HasPrefix(result, regular), "admin help must extend the regular help")
	assert.Contains(t, result, adminSectionHeader)
	assert.Contains(t, result, testCommandsHeader)
	assert.Less(t, strings.Index(result, adminSectionHeader), strings.Index(result, testCommandsHeader))

	for _, command := range []string{
		constants.EventStartCommand,
		constants.EventSetupCommand,
		constants.EventEditCommand,
		constants.EventDeleteCommand,
		constants.ShowTopicsCommand,
		constants.CodeCommand,
		constants.AdminProfilesCommand,
		constants.TrySummarizeCommand,
		constants.TryCreateCoffeePoolCommand,
		constants.TryGenerateCoffeePairsCommand,
		constants.TryLinkToLearnCommand,
	} {
		assert.Contains(t, result, "└ /"+command+" - ", "admin help must describe /%s", command)
	}
}

func TestFormatHelpMessage_UsesConfigForRandomCoffeeLink(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 42, RandomCoffeeTopicID: 7}

	result := FormatHelpMessage(false, cfg)

	assert.Contains(t, result, `href="https://telegram.me/c/42/7"`)
	assert.NotContains(t, result, "1234567890")
}
