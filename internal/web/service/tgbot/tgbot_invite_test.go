package tgbot

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initInviteDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func seedClient(t *testing.T, email, subID string, tgID int64) {
	t.Helper()
	rec := &model.ClientRecord{Email: email, SubID: subID, TgID: tgID, Enable: true}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
}

// A SubID doubles as the invite token, so binding must be first-claim-wins:
// an unclaimed client binds, the owner re-tapping is idempotent, and a client
// already held by someone else must never be reassigned by a stranger.
func TestResolveInviteToken(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "unclaimed@x", "subfree0000000001", 0)
	seedClient(t, "owned@x", "subowned000000002", 5150)

	tg := &Tgbot{}

	tests := []struct {
		name  string
		token string
		from  int64
		want  inviteOutcome
		email string
	}{
		{"unclaimed binds", "subfree0000000001", 777, inviteBindable, "unclaimed@x"},
		{"owner is idempotent", "subowned000000002", 5150, inviteAlreadyOwned, "owned@x"},
		{"stranger is refused", "subowned000000002", 999, inviteTaken, "owned@x"},
		{"unknown token", "nosuchtoken000000", 777, inviteInvalid, ""},
		{"empty token", "", 777, inviteInvalid, ""},
		{"blank token", "   ", 777, inviteInvalid, ""},
		{"missing sender id", "subfree0000000001", 0, inviteInvalid, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, recs := tg.resolveInviteToken(tc.token, tc.from)
			if got != tc.want {
				t.Fatalf("outcome = %v, want %v", got, tc.want)
			}
			if tc.email == "" {
				if len(recs) != 0 {
					t.Fatalf("records = %+v, want none", recs)
				}
				return
			}
			if len(recs) != 1 {
				t.Fatalf("resolved %d records, want exactly 1", len(recs))
			}
			if recs[0].Email != tc.email {
				t.Fatalf("record.Email = %q, want %q", recs[0].Email, tc.email)
			}
		})
	}
}

// Whitespace around a token is common when a link is copied by hand; it must
// still resolve rather than being reported as an invalid invite.
func TestResolveInviteTokenTrimsWhitespace(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "trim@x", "subtrim0000000003", 0)

	tg := &Tgbot{}
	got, recs := tg.resolveInviteToken("  subtrim0000000003\n", 42)
	if got != inviteBindable {
		t.Fatalf("outcome = %v, want inviteBindable", got)
	}
	if len(recs) != 1 || recs[0].Email != "trim@x" {
		t.Fatalf("records = %+v, want trim@x", recs)
	}
}

// A subscription can span several clients, so a token that maps to more than
// one must bind every unbound part — otherwise the customer receives one config
// and silently loses the rest.
func TestClaimInviteBindsEveryClientSharingSubID(t *testing.T) {
	initInviteDB(t)
	const shared = "subshared00000003"
	seedClient(t, "multi-a@x", shared, 0)
	seedClient(t, "multi-b@x", shared, 0)
	seedClient(t, "multi-c@x", shared, 0)

	tg := &Tgbot{}
	outcome, records := tg.resolveInviteToken(shared, 7000)
	if outcome != inviteBindable {
		t.Fatalf("outcome = %v, want inviteBindable", outcome)
	}
	if len(records) != 3 {
		t.Fatalf("resolved %d records, want all 3 sharing the subId", len(records))
	}
}

// A token part-owned by a third party must not be claimable: one stranger
// holding a slice of a shared subscription blocks the whole token.
func TestResolveInviteTokenRefusesPartiallyTakenSubID(t *testing.T) {
	initInviteDB(t)
	const shared = "subpartial0000004"
	seedClient(t, "part-free@x", shared, 0)
	seedClient(t, "part-held@x", shared, 9999)

	tg := &Tgbot{}
	if outcome, _ := tg.resolveInviteToken(shared, 7001); outcome != inviteTaken {
		t.Fatalf("outcome = %v, want inviteTaken", outcome)
	}
}

// The vague public reply is right for a stranger and useless for an admin, who
// needs to know which account is holding the token.
func TestInviteDiagnosis(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "diag@x", "subdiag0000000005", 4242)
	tg := &Tgbot{}

	// Without a localizer I18nBot echoes the key and drops its parameters, so
	// the holder detail is asserted on the helper that builds it.
	_, records := tg.resolveInviteToken("subdiag0000000005", 7002)
	holders := inviteHolders(records)
	if !strings.Contains(holders, "diag@x") || !strings.Contains(holders, "4242") {
		t.Fatalf("holders %q should name the client and the holding Telegram id", holders)
	}
	if key := tg.inviteDiagnosis("subdiag0000000005", records); key != "tgbot.messages.inviteDiagTaken" {
		t.Fatalf("diagnosis key = %q, want the already-bound key", key)
	}

	_, none := tg.resolveInviteToken("nosuchtoken000006", 7002)
	if inviteHolders(none) != "" {
		t.Fatal("an unknown token has no holders")
	}
	if key := tg.inviteDiagnosis("nosuchtoken000006", none); key != "tgbot.messages.inviteDiagUnknown" {
		t.Fatalf("diagnosis key = %q, want the unknown-token key", key)
	}
}

// One config per Telegram account: without this a customer could collect other
// people's subscriptions simply by collecting their invite links.
func TestHoldsAnyClient(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "already@x", "subheld0000000007", 8100)
	seedClient(t, "offered@x", "suboffer000000008", 0)

	tg := &Tgbot{}
	_, offered := tg.resolveInviteToken("suboffer000000008", 8100)

	if !tg.holdsAnyClient(8100, offered) {
		t.Fatal("an account already holding an unrelated client must be blocked")
	}
	if tg.holdsAnyClient(8101, offered) {
		t.Fatal("an account holding nothing must be allowed to claim")
	}
}

// Re-tapping the link for a subscription the caller already partly holds must
// still complete the binding rather than being read as a second config.
func TestHoldsAnyClientIgnoresTheTokenBeingClaimed(t *testing.T) {
	initInviteDB(t)
	const shared = "subresume00000009"
	seedClient(t, "resume-a@x", shared, 8200)
	seedClient(t, "resume-b@x", shared, 0)

	tg := &Tgbot{}
	_, records := tg.resolveInviteToken(shared, 8200)
	if tg.holdsAnyClient(8200, records) {
		t.Fatal("records behind the claimed token must not count against the cap")
	}
}
