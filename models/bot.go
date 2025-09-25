package models

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type Bot struct {
	Bot   *tgbotapi.BotAPI
	Chats map[int64]*Chat
}
