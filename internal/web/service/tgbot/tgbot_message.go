package tgbot

import (
	"html"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	statePmText      = "awaiting_pm_text"
	stateReplyPrefix = "awaiting_reply:"
)

// The reply target rides in the state string so a single map entry survives the
// admin typing an unrelated message in between.
func parseReplyTarget(state string) (int64, bool) {
	if !strings.HasPrefix(state, stateReplyPrefix) {
		return 0, false
	}
	target, err := strconv.ParseInt(strings.TrimPrefix(state, stateReplyPrefix), 10, 64)
	if err != nil || target == 0 {
		return 0, false
	}
	return target, true
}

// Conversational flows added on top of the add-client wizard's states. Returns
// true when the state was consumed so the caller leaves its own switch alone.
func (t *Tgbot) handleConversationState(message *telego.Message, state string) bool {
	chatId := message.Chat.ID
	text := strings.TrimSpace(message.Text)

	switch {
	case state == stateBroadcast:
		userStateMgr.clear(chatId)
		if !checkAdmin(message.From.ID) {
			return true
		}
		t.previewBroadcast(chatId, text)
		return true
	case state == stateHelpText:
		userStateMgr.clear(chatId)
		if !checkAdmin(message.From.ID) {
			return true
		}
		t.saveHelpText(chatId, text)
		return true
	case state == statePmText:
		userStateMgr.clear(chatId)
		t.forwardClientMessage(message, text)
		return true
	case strings.HasPrefix(state, stateReplyPrefix):
		userStateMgr.clear(chatId)
		target, ok := parseReplyTarget(state)
		if !ok || !checkAdmin(message.From.ID) {
			return true
		}
		t.deliverAdminReply(chatId, target, text)
		return true
	}
	return false
}

// Only clients already bound to a Telegram account may message admins, which
// keeps the admin inbox free of traffic from arbitrary strangers.
func (t *Tgbot) clientEmailsFor(tgUserID int64) []string {
	records, err := t.clientService.GetRecordsByTgID(tgUserID)
	if err != nil || len(records) == 0 {
		return nil
	}
	emails := make([]string, 0, len(records))
	for _, r := range records {
		emails = append(emails, r.Email)
	}
	return emails
}

func (t *Tgbot) startClientMessage(message *telego.Message, text string) {
	chatId := message.Chat.ID
	if len(t.clientEmailsFor(message.From.ID)) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmNotLinked"))
		return
	}
	if text == "" {
		userStateMgr.set(chatId, statePmText)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmPrompt"))
		return
	}
	t.forwardClientMessage(message, text)
}

func (t *Tgbot) forwardClientMessage(message *telego.Message, text string) {
	chatId := message.Chat.ID
	emails := t.clientEmailsFor(message.From.ID)
	if len(emails) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmNotLinked"))
		return
	}
	if text == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.pmUsage"))
		return
	}

	name := html.EscapeString(message.From.FirstName)
	if message.From.Username != "" {
		name += " (@" + html.EscapeString(message.From.Username) + ")"
	}
	header := t.I18nBot("tgbot.messages.pmHeader", "Name=="+name, "Clients=="+html.EscapeString(strings.Join(emails, ", ")))

	replyKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.replyToClient")).
				WithCallbackData(t.encodeQuery("pm_reply " + strconv.FormatInt(chatId, 10))),
		),
	)
	for _, adminId := range adminIds {
		t.SendMsgToTgbot(adminId, header+"\r\n"+html.EscapeString(text), replyKeyboard)
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.pmSent"))
}

func (t *Tgbot) promptAdminReply(chatId int64, target string) {
	if _, err := strconv.ParseInt(target, 10, 64); err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.sendUsage"))
		return
	}
	userStateMgr.set(chatId, stateReplyPrefix+target)
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.replyPrompt"))
}

func (t *Tgbot) deliverAdminReply(adminChatId int64, target int64, text string) {
	if text == "" {
		t.SendMsgToTgbot(adminChatId, t.I18nBot("tgbot.commands.sendUsage"))
		return
	}
	if err := t.sendDirect(target, text); err != nil {
		t.SendMsgToTgbot(adminChatId, t.I18nBot("tgbot.messages.replyFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(adminChatId, t.I18nBot("tgbot.messages.replyDelivered"))
}
