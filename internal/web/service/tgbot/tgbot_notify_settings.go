package tgbot

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// One notice the admin can switch off. Read and write stay together so a new
// notice cannot be listed in the menu without being wired to its setting.
type botNotification struct {
	callback string
	labelKey string
	get      func(*Tgbot) (bool, error)
	set      func(*Tgbot, bool) error
}

var botNotifications = []botNotification{
	{
		callback: "notify_usage",
		labelKey: "tgbot.buttons.notifyServerUsage",
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyServerUsage() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyServerUsage(v) },
	},
	{
		callback: "notify_deplete",
		labelKey: "tgbot.buttons.notifyDepleteSoon",
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyDepleteSoon() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyDepleteSoon(v) },
	},
	{
		callback: "notify_new_client",
		labelKey: "tgbot.buttons.notifyNewClient",
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyNewClient() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyNewClient(v) },
	},
	{
		callback: "notify_quota",
		labelKey: "tgbot.buttons.notifyQuota",
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotNotifyQuota() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotNotifyQuota(v) },
	},
	{
		callback: "notify_backup",
		labelKey: "tgbot.buttons.notifyBackup",
		get:      func(t *Tgbot) (bool, error) { return t.settingService.GetTgBotBackup() },
		set:      func(t *Tgbot, v bool) error { return t.settingService.SetTgBotBackup(v) },
	},
}

func notificationByCallback(callback string) (botNotification, bool) {
	for _, notification := range botNotifications {
		if notification.callback == callback {
			return notification, true
		}
	}
	return botNotification{}, false
}

// A lookup failure shows the notice as on, matching SendReport's fail-open read:
// the menu must not claim a notice is off while the job still sends it.
func (t *Tgbot) notificationToggleLabel(notification botNotification) string {
	enabled, err := notification.get(t)
	if err != nil {
		logger.Warning("tgbot: notification setting lookup failed:", err)
		enabled = true
	}
	mark := "❌"
	if enabled {
		mark = "✅"
	}
	return mark + " " + t.I18nBot(notification.labelKey)
}

func (t *Tgbot) notificationsKeyboard() *telego.InlineKeyboardMarkup {
	rows := make([][]telego.InlineKeyboardButton, 0, len(botNotifications)+1)
	for _, notification := range botNotifications {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.notificationToggleLabel(notification)).
				WithCallbackData(t.encodeQuery("notify_toggle "+notification.callback)),
		))
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_reports")),
	))
	return tu.InlineKeyboard(rows...)
}

func (t *Tgbot) notificationsMenu(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.notifications"), t.notificationsKeyboard())
}

func (t *Tgbot) toggleNotification(chatId int64, callback string, messageID int) {
	notification, found := notificationByCallback(callback)
	if !found {
		return
	}
	enabled, err := notification.get(t)
	if err != nil {
		logger.Warning("tgbot: notification setting lookup failed:", err)
		enabled = true
	}
	if err := notification.set(t, !enabled); err != nil {
		logger.Warning("tgbot: notification setting save failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if messageID > 0 {
		t.editMessageCallbackTgBot(chatId, messageID, t.notificationsKeyboard())
		return
	}
	t.notificationsMenu(chatId)
}
