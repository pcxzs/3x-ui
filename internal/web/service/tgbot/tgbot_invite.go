package tgbot

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type inviteOutcome int

const (
	inviteInvalid inviteOutcome = iota
	inviteTaken
	inviteAlreadyOwned
	inviteBindable
)

// A client's SubID doubles as its invite token. Unknown and already-claimed
// tokens share one reply so a prober cannot tell valid SubIDs from invalid ones.
func (t *Tgbot) resolveInviteToken(token string, fromID int64) (inviteOutcome, *model.ClientRecord) {
	token = strings.TrimSpace(token)
	if token == "" || fromID <= 0 {
		return inviteInvalid, nil
	}
	record, err := t.clientService.GetRecordBySubID(token)
	if err != nil || record == nil {
		return inviteInvalid, nil
	}
	switch record.TgID {
	case 0:
		return inviteBindable, record
	case fromID:
		return inviteAlreadyOwned, record
	default:
		return inviteTaken, record
	}
}

func (t *Tgbot) claimInvite(chatId int64, fromID int64, token string) {
	outcome, record := t.resolveInviteToken(token, fromID)
	switch outcome {
	case inviteAlreadyOwned:
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+record.Email))
	case inviteBindable:
		if err := t.bindRecordToUser(record, fromID); err != nil {
			logger.Warning("tgbot: invite bind failed:", err)
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
			return
		}
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteBound", "Email=="+record.Email))
	default:
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteInvalid"))
	}
}

func (t *Tgbot) bindRecordToUser(record *model.ClientRecord, tgID int64) error {
	traffic, err := t.inboundService.GetClientTrafficByEmail(record.Email)
	if err != nil {
		return err
	}
	if traffic == nil {
		return common.NewError("no traffic record for client:", record.Email)
	}
	needRestart, err := t.clientService.SetClientTelegramUserID(&t.inboundService, traffic.Id, tgID)
	if needRestart {
		t.xrayService.SetToNeedRestart()
	}
	return err
}

func (t *Tgbot) inviteLinkFor(email string) (string, error) {
	record, err := t.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return "", err
	}
	if record.SubID == "" {
		return "", common.NewError("client has no subId:", email)
	}
	username := botUsername()
	if username == "" {
		return "", common.NewError("bot username unavailable")
	}
	return "https://t.me/" + username + "?start=" + record.SubID, nil
}

func (t *Tgbot) sendInviteLink(chatId int64, email string) {
	link, err := t.inviteLinkFor(email)
	if err != nil {
		logger.Warning("tgbot: invite link failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inviteLink", "Email=="+email, "Link=="+link))
}
