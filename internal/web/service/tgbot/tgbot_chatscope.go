package tgbot

import "github.com/mymmrac/telego"

// The bot serves one Telegram account per conversation, but every authorization
// check keys on the sender while conversation state keys on the chat. Those are
// the same identity only in a private chat, so group traffic is dropped at the
// door rather than being made safe handler by handler.
func isPrivateChat(chat telego.Chat) bool {
	return chat.Type == telego.ChatTypePrivate
}
