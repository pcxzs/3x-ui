package tgbot

import (
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	bulkExtend  = "extend"
	bulkDisable = "disable"

	// Extending everyone on the ladder is the point; extending a whole panel by
	// accident is not, so a larger match refuses instead of half-applying.
	bulkLimit = 50
	// The one duration offered. Anything else belongs on the per-client menu,
	// where the admin can see who they are changing.
	bulkExtendDays = 30
)

type bulkClient = service.ClientWithAttachments

// extendedExpiry mirrors reset_exp_c's arithmetic. A negative expiry is the
// panel's marker for "N days from first use", not a date in the past.
func extendedExpiry(current int64, days int64, now time.Time) int64 {
	const dayMs = int64(24 * 60 * 60000)
	if current > 0 {
		if current-now.UnixMilli() < 0 {
			return -(days * dayMs)
		}
		return current + days*dayMs
	}
	return current - days*dayMs
}

// bulkTargets names exactly who an action would touch. The second return says
// the match was too large to apply, in which case nothing is selected.
func bulkTargets(clients []bulkClient, action string, now time.Time) ([]bulkClient, bool) {
	var keep func(*bulkClient) bool
	switch action {
	case bulkExtend:
		keep = func(c *bulkClient) bool { return expiryRung(c.ExpiryTime, now) != rungNone }
	case bulkDisable:
		// Only those still on: a disabled client is already where this action
		// would put them, and toggling would switch them back.
		keep = func(c *bulkClient) bool { return c.Enable && expiryRung(c.ExpiryTime, now) == rungOverdue }
	default:
		return nil, false
	}

	targets := make([]bulkClient, 0, len(clients))
	for i := range clients {
		if keep(&clients[i]) {
			targets = append(targets, clients[i])
		}
	}
	if len(targets) > bulkLimit {
		return nil, true
	}
	return targets, false
}

func (t *Tgbot) bulkKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.bulkExtend")).WithCallbackData(t.encodeQuery("bulk_preview "+bulkExtend)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.bulkDisable")).WithCallbackData(t.encodeQuery("bulk_preview "+bulkDisable)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_clients")),
		),
	)
}

func (t *Tgbot) bulkMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.bulkMenu"), t.bulkKeyboard())
}

// The preview names every client before anything is applied, so the confirm is
// a decision rather than a guess.
func (t *Tgbot) bulkPreview(chatId int64, action string) {
	targets, tooMany := t.resolveBulk(chatId, action)
	if targets == nil {
		if !tooMany {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.bulkNoTargets"), t.bulkKeyboard())
		}
		return
	}

	messageKey := "tgbot.messages.bulkPreviewExtend"
	if action == bulkDisable {
		messageKey = "tgbot.messages.bulkPreviewDisable"
	}
	t.SendMsgToTgbot(chatId,
		t.I18nBot(messageKey,
			"Count=="+strconv.Itoa(len(targets)),
			"Days=="+strconv.Itoa(bulkExtendDays),
			"Clients=="+strings.Join(emailList(targets), ", ")),
		tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.bulkConfirm")).WithCallbackData(t.encodeQuery("bulk_apply "+action)),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("bulk_menu")),
			),
		))
}

// Re-resolved from the database rather than carried in the callback payload,
// so the confirm applies to who matches now, not to a list a button asserted.
func (t *Tgbot) bulkApply(chatId int64, action string) {
	targets, tooMany := t.resolveBulk(chatId, action)
	if targets == nil {
		if !tooMany {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.bulkNoTargets"), t.bulkKeyboard())
		}
		return
	}

	now := time.Now()
	lines := make([]string, 0, len(targets))
	for i := range targets {
		email := targets[i].Email
		var err error
		var needRestart bool
		switch action {
		case bulkExtend:
			needRestart, err = t.clientService.ResetClientExpiryTimeByEmail(&t.inboundService, email,
				extendedExpiry(targets[i].ExpiryTime, bulkExtendDays, now))
		case bulkDisable:
			_, needRestart, err = t.clientService.ToggleClientEnableByEmail(&t.inboundService, email)
		}
		if needRestart {
			t.xrayService.SetToNeedRestart()
		}
		mark := deliveryDelivered
		if err != nil {
			logger.Warning("tgbot: bulk", action, "failed for", email, ":", err)
			mark = deliveryFailed
		}
		lines = append(lines, mark.mark()+email)
	}

	t.SendMsgToTgbot(chatId,
		t.I18nBot("tgbot.messages.bulkDone",
			"Count=="+strconv.Itoa(len(lines)),
			"Clients=="+strings.Join(lines, ", ")),
		t.bulkKeyboard())
}

func (t *Tgbot) resolveBulk(chatId int64, action string) ([]bulkClient, bool) {
	clients, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: bulk client list failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return nil, true
	}

	targets, tooMany := bulkTargets(clients, action, time.Now())
	if tooMany {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.bulkTooMany", "Limit=="+strconv.Itoa(bulkLimit)), t.bulkKeyboard())
		return nil, true
	}
	if len(targets) == 0 {
		return nil, false
	}
	return targets, false
}

func emailList(clients []bulkClient) []string {
	emails := make([]string, 0, len(clients))
	for i := range clients {
		emails = append(emails, clients[i].Email)
	}
	return emails
}
