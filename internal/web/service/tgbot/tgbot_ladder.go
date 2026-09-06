package tgbot

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// renewalRung is how close a subscription is to its renewal date. The rungs are
// exact-day matches rather than a window, so each one fires once per client.
type renewalRung int

const (
	rungNone renewalRung = iota
	rungThreeDays
	rungTwoDays
	rungTomorrow
	rungToday
	rungOverdue
)

// A rung missing from this map is summarised for admins but never sent to the
// customer: rungTwoDays exists to widen the admin's view, not to add a reminder.
var rungMessageKey = map[renewalRung]string{
	rungThreeDays: "tgbot.messages.renewThreeDays",
	rungTomorrow:  "tgbot.messages.renewTomorrow",
	rungToday:     "tgbot.messages.renewToday",
	rungOverdue:   "tgbot.messages.renewOverdue",
}

// A zero expiry never lapses and a negative one has not started counting, so
// neither is on the ladder. Days are compared as calendar days, not as 24-hour
// blocks, so "tomorrow" means the next date rather than the next 24 hours.
func expiryRung(expiryTime int64, now time.Time) renewalRung {
	if expiryTime <= 0 {
		return rungNone
	}
	expiry := time.UnixMilli(expiryTime)
	days := int(dayStart(expiry).Sub(dayStart(now)).Hours() / 24)
	switch {
	case days < 0:
		return rungOverdue
	case days == 0:
		return rungToday
	case days == 1:
		return rungTomorrow
	case days == 2:
		return rungTwoDays
	case days == 3:
		return rungThreeDays
	default:
		return rungNone
	}
}

func dayStart(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// Exact-day rungs assume one pass per day, but tgRunTime is an admin-editable
// cron: an hourly schedule would otherwise nag every customer 24 times.
func (t *Tgbot) ladderAlreadyRanToday(today string) bool {
	last, err := t.settingService.GetTgBotLadderRun()
	if err != nil {
		logger.Warning("tgbot: renewal ladder state lookup failed:", err)
		return false
	}
	return last == today
}

func (t *Tgbot) notifyRenewals() {
	today := time.Now().Format("2006-01-02")
	if t.ladderAlreadyRanToday(today) {
		return
	}

	records, err := t.clientService.List()
	if err != nil {
		logger.Warning("tgbot: renewal ladder client list failed:", err)
		return
	}

	now := time.Now()
	byRung := map[renewalRung][]string{}
	for i := range records {
		record := &records[i].ClientRecord
		rung := expiryRung(record.ExpiryTime, now)
		if rung == rungNone {
			continue
		}
		byRung[rung] = append(byRung[rung], record.Email)
		if messageKey, notifies := rungMessageKey[rung]; notifies && record.TgID != 0 {
			t.SendMsgToTgbot(record.TgID, t.I18nBot(messageKey, "Email=="+record.Email))
		}
	}

	if summary := t.renewalSummary(byRung); summary != "" {
		t.SendMsgToTgbotAdmins(summary)
	}
	if err := t.settingService.SetTgBotLadderRun(today); err != nil {
		logger.Warning("tgbot: renewal ladder state save failed:", err)
	}
}

// Groups the day's notices so an admin sees who was chased without reading the
// same message once per customer.
func (t *Tgbot) renewalSummary(byRung map[renewalRung][]string) string {
	order := []renewalRung{rungOverdue, rungToday, rungTomorrow, rungTwoDays, rungThreeDays}
	sections := make([]string, 0, len(order))
	for _, rung := range order {
		emails := byRung[rung]
		if len(emails) == 0 {
			continue
		}
		sort.Strings(emails)
		sections = append(sections, t.I18nBot(rungSummaryKey[rung],
			"Count=="+strconv.Itoa(len(emails)),
			"Clients=="+strings.Join(emails, ", ")))
	}
	if len(sections) == 0 {
		return ""
	}
	return t.I18nBot("tgbot.messages.renewSummaryHeader") + "\r\n\r\n" + strings.Join(sections, "\r\n")
}

var rungSummaryKey = map[renewalRung]string{
	rungThreeDays: "tgbot.messages.renewSummaryThreeDays",
	rungTwoDays:   "tgbot.messages.renewSummaryTwoDays",
	rungTomorrow:  "tgbot.messages.renewSummaryTomorrow",
	rungToday:     "tgbot.messages.renewSummaryToday",
	rungOverdue:   "tgbot.messages.renewSummaryOverdue",
}
