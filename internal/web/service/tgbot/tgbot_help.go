package tgbot

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	stateHelpText = "awaiting_help_text"
	helpTextReset = "-"
)

// An empty stored value means the operator never customised the text, so the
// translated built-in help is served instead of a blank message.
func (t *Tgbot) helpText() string {
	stored, err := t.settingService.GetTgBotHelpText()
	if err != nil {
		logger.Warning("tgbot: help text lookup failed:", err)
		return t.I18nBot("tgbot.commands.help")
	}
	if strings.TrimSpace(stored) == "" {
		return t.I18nBot("tgbot.commands.help")
	}
	return stored
}

func (t *Tgbot) startSetHelp(chatId int64) {
	userStateMgr.set(chatId, stateHelpText)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextPrompt", "Reset=="+helpTextReset))
}

// The text is proved renderable before it is stored: the bot sends every
// message with HTML parse mode, so stray markup would silently break /help.
func (t *Tgbot) saveHelpText(chatId int64, text string) {
	if strings.TrimSpace(text) == helpTextReset {
		if err := t.settingService.SetTgBotHelpText(""); err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextFailed", "Error=="+err.Error()))
			return
		}
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextReset"))
		return
	}
	if strings.TrimSpace(text) == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextEmpty"))
		return
	}
	if err := t.sendHTMLDirect(chatId, text); err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextInvalid", "Error=="+err.Error()))
		return
	}
	if err := t.settingService.SetTgBotHelpText(text); err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.helpTextSaved"))
}
