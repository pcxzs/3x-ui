package tgbot

import (
	"strings"
	"testing"
)

// Callback data is attacker-controlled, so any button that names a client must
// resolve through clientSelfAction — the gate that re-checks ownership. A new
// targeted callback added without that gate is an IDOR, and fails here.
func TestTargetedClientCallbacksAreOwnershipChecked(t *testing.T) {
	initLangDB(t)
	tg := &Tgbot{}
	const email = "amy@example.com"

	rendered := map[string][]string{
		"renew keyboard":  callbackData(tg.renewKeyboard(email)),
		"client keyboard": callbackData(tg.clientKeyboard(levelClient)),
		"settings":        callbackData(tg.settingsKeyboard()),
		"languages":       callbackData(tg.languageKeyboard("")),
	}

	for source, data := range rendered {
		for _, entry := range data {
			verb, arg, matched := clientSelfAction(entry)
			switch {
			case matched:
				if got := clientSelfTarget(verb, arg); got != email {
					t.Fatalf("%s: %q resolves to target %q, want %q", source, entry, got, email)
				}
			case strings.Contains(entry, email):
				t.Fatalf("%s: %q names a client but is not ownership-checked", source, entry)
			}
			if !isClientSelfCallback(entry) {
				t.Fatalf("%s: %q is rendered for a customer but the level gate rejects it", source, entry)
			}
		}
	}
}

// The two lists are the level gate and the ownership check. A prefix in one and
// not the other is admitted by one and forgotten by the other.
func TestClientSelfPrefixesPassTheLevelGate(t *testing.T) {
	for _, prefix := range clientSelfPrefixes {
		data := prefix + "amy@example.com"
		if !isClientSelfCallback(data) {
			t.Fatalf("prefix %q is ownership-checked but the level gate rejects it", prefix)
		}
		verb, arg, ok := clientSelfAction(data)
		if !ok {
			t.Fatalf("prefix %q does not match its own action parser", prefix)
		}
		if clientSelfTarget(verb, arg) == "" {
			t.Fatalf("prefix %q resolves to an empty target, so ownership cannot be checked", prefix)
		}
	}
}

// ownsClient is the whole check. An empty or zero caller must never pass, or a
// callback with a stripped sender would read as owning everything.
func TestOwnsClientRejectsMissingIdentity(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	tests := []struct {
		name  string
		tgID  int64
		email string
	}{
		{name: "no telegram id", tgID: 0, email: "amy@example.com"},
		{name: "negative telegram id", tgID: -1, email: "amy@example.com"},
		{name: "no email", tgID: 42, email: ""},
		{name: "unbound client", tgID: 42, email: "amy@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tg.ownsClient(tc.tgID, tc.email) {
				t.Fatalf("ownsClient(%d, %q) = true, want false", tc.tgID, tc.email)
			}
		})
	}
}
