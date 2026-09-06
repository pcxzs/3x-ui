package tgbot

import (
	"path/filepath"
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
			got, rec := tg.resolveInviteToken(tc.token, tc.from)
			if got != tc.want {
				t.Fatalf("outcome = %v, want %v", got, tc.want)
			}
			if tc.email == "" {
				if rec != nil {
					t.Fatalf("record = %+v, want nil", rec)
				}
				return
			}
			if rec == nil {
				t.Fatal("record = nil, want a client record")
			}
			if rec.Email != tc.email {
				t.Fatalf("record.Email = %q, want %q", rec.Email, tc.email)
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
	got, rec := tg.resolveInviteToken("  subtrim0000000003\n", 42)
	if got != inviteBindable {
		t.Fatalf("outcome = %v, want inviteBindable", got)
	}
	if rec == nil || rec.Email != "trim@x" {
		t.Fatalf("record = %+v, want trim@x", rec)
	}
}
