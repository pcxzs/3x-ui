package tgbot

import (
	"slices"
	"strings"
)

// Per-client callbacks a customer may send for their OWN client. The list is
// the single source of truth for both the level gate and the ownership check,
// so a new action cannot be admitted by one and forgotten by the other.
var clientSelfPrefixes = []string{
	"client_sub_links ",
	"client_individual_links ",
	"client_qr_links ",
	"qr_sub ",
	"qr_subjson ",
	"qr_pick ",
	"qr_one ",
	"renew_req ",
	"renew_mute ",
	"client_reset_self ",
	"client_reset_self_c ",
}

func clientSelfAction(data string) (verb string, arg string, ok bool) {
	for _, prefix := range clientSelfPrefixes {
		if rest, found := strings.CutPrefix(data, prefix); found {
			return strings.TrimSuffix(prefix, " "), rest, true
		}
	}
	return "", "", false
}

// The target is always the first field; qr_one carries "<email> <index>".
func clientSelfTarget(verb, arg string) string {
	if verb == "qr_one" {
		email, _, _ := strings.Cut(strings.TrimSpace(arg), " ")
		return email
	}
	return strings.TrimSpace(arg)
}

// Callback data is attacker-controlled. Telegram's MTProto layer lets a user's
// own client post arbitrary bytes against any message the bot sent, so the fact
// that we rendered a button is no evidence of what comes back: the target is
// re-checked against the caller's own clients on every request.
func (t *Tgbot) ownsClient(tgUserID int64, email string) bool {
	if email == "" || tgUserID <= 0 {
		return false
	}
	return slices.Contains(t.clientEmailsFor(tgUserID), email)
}
