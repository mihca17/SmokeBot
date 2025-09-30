package main

import (
	"database/sql"
	"fmt"
	"log"
	"smoke-bot/database/repository"
	"smoke-bot/logger"
	"smoke-bot/models"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "modernc.org/sqlite"
)

type Controller struct {
	bot *models.Bot
}

func NewController(bot *models.Bot) *Controller {
	return &Controller{bot: bot}
}

var (
	mu           sync.Mutex
	allowedChats = map[int64]bool{
		-4845216092:    true,
		-1002652733052: true,
	}
)

func isOldMessage(message *tgbotapi.Message) bool {
	messageTime := time.Unix(int64(message.Date), 0)
	botStartTime := time.Now().Add(-5 * time.Minute) // Допускаем небольшую погрешность

	return messageTime.Before(botStartTime)
}

func isChatAllowed(chatID int64) bool {
	_, exists := allowedChats[chatID]
	return exists
}

func (c Controller) handleConsent(message *tgbotapi.Message, repo *repository.SQLiteRepository) {
	_, err := repo.GetByID(message.From.ID)
	if err != sql.ErrNoRows {
		logger.Error("Пользователь уже существует", err)
		answer := fmt.Sprintf("❌ Пользоватль @%s уже в базе", message.From.UserName)
		c.sendMessage(message.Chat.ID, answer)
		return
	}

	err = repo.SaveUser(message.From.ID, message.From.UserName, message.Chat.ID)
	if err != nil {
		logger.Error("Ошибка сохранения пользователя", err)
		return
	}
	answer := fmt.Sprintf("✅ @%s теперь с нами!", message.From.UserName)
	c.sendMessage(message.Chat.ID, answer)
}

func (c Controller) handleSmoke(message *tgbotapi.Message, repo *repository.SQLiteRepository) {
	mu.Lock()
	defer mu.Unlock()

	if c.bot.Chats[message.Chat.ID].ActiveSmoke {
		answer := fmt.Sprintf("❌ @%s, уже кто-то предложил перекур!", message.From.UserName)
		c.sendMessage(message.Chat.ID, answer)
		return
	}

	_, err := repo.GetByID(message.From.ID)
	if err == sql.ErrNoRows {
		logger.Info("Попытка создания перекура незарегистрированным пользователем")
		answer := fmt.Sprintf("❌ @%s, для предложения перекура вам неободимо пустить по вене", message.From.UserName)
		c.sendMessage(message.Chat.ID, answer)
		return
	}

	c.bot.Chats[message.Chat.ID].ActiveSmoke = true
	c.bot.Chats[message.Chat.ID].SmokeSession.SmokeStarter = message.From.ID

	// Пользователи, давшие согласие
	var users []string

	allUsers, err := repo.GetAllUsers()
	if err != nil {
		logger.Error("Ошибка получения пользователей", err)
		return
	}

	for _, user := range allUsers {
		users = append(users, fmt.Sprintf("@%s", user.Username))
	}

	if len(users) == 0 {
		c.sendMessage(message.Chat.ID, "❌ Пока никто не добавился ;(")
		c.bot.Chats[message.Chat.ID].ActiveSmoke = false
		return
	}

	c.bot.Chats[message.Chat.ID].SmokeSession.OriginalSmokeText = fmt.Sprintf("🚬 @%s предлагает перекур! \nПриглашённые джентльмены: %s",
		message.From.UserName, getUsers(users))

	msg := tgbotapi.NewMessage(message.Chat.ID, c.bot.Chats[message.Chat.ID].SmokeSession.OriginalSmokeText)

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да", "yes"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Нет", "no"),
		),
	)
	sentMsg, err := c.bot.Bot.Send(msg)
	if err != nil {
		log.Printf("Ошибка отправки сообщения с кнопками: %v", err)
		c.bot.Chats[message.Chat.ID].ActiveSmoke = false
		return
	}
	c.bot.Chats[message.Chat.ID].SmokeSession.SmokeMessageID = int64(sentMsg.MessageID)
	//sendMessage(message.Chat.ID, msg.Text)
}

func (c Controller) handleCallback(callback *tgbotapi.CallbackQuery) {
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	c.bot.Bot.Send(callbackConfig)

	if callback.Data == "yes" || callback.Data == "no" {
		c.handleButtonResponse(callback)
	}
}

func (c Controller) handleButtonResponse(callback *tgbotapi.CallbackQuery) {
	mu.Lock()
	defer mu.Unlock()

	chatId := callback.Message.Chat.ID

	if !c.bot.Chats[chatId].ActiveSmoke {
		return
	}

	if callback.Data == "yes" {
		userName := fmt.Sprintf("@%s", callback.From.UserName)

		for _, user := range c.bot.Chats[chatId].SmokeSession.JoinedUsers {
			if user == userName {
				return
			}
		}
		c.bot.Chats[chatId].SmokeSession.JoinedUsers = append(c.bot.Chats[chatId].SmokeSession.JoinedUsers, userName)

		// Редактируем сообщение с обновленным списком
		c.editMessage(callback.Message.Chat.ID, c.bot.Chats[chatId].SmokeSession.SmokeMessageID)
	} else if callback.Data == "no" {
		userName := fmt.Sprintf("@%s", callback.From.UserName)

		joinedUsers2 := make([]string, 0)
		for _, user := range c.bot.Chats[chatId].SmokeSession.JoinedUsers {
			if user == userName {
				continue
			}
			joinedUsers2 = append(joinedUsers2, user)
		}
		c.bot.Chats[chatId].SmokeSession.JoinedUsers = joinedUsers2

		c.editMessage(callback.Message.Chat.ID, c.bot.Chats[chatId].SmokeSession.SmokeMessageID)
	}
}

func (c Controller) editMessage(chatID int64, messageID int64) {
	newText := fmt.Sprintf("%s\n\nПрисоединились: %s",
		c.bot.Chats[chatID].SmokeSession.OriginalSmokeText,
		getUsers(c.bot.Chats[chatID].SmokeSession.JoinedUsers),
	)

	if len(c.bot.Chats[chatID].SmokeSession.JoinedUsers) == 0 {
		newText += "пока никто"
	}

	// Редактируем сообщение
	edit := tgbotapi.NewEditMessageText(chatID, int(messageID), newText)
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("✅ Да", "yes"),
				tgbotapi.NewInlineKeyboardButtonData("❌ Нет", "no"),
			},
		},
	}
	c.bot.Bot.Send(edit)
}

func (c Controller) handleEndSmoke(message *tgbotapi.Message) {
	mu.Lock()
	defer mu.Unlock()

	if c.bot.Chats[message.Chat.ID].ActiveSmoke && message.From.ID == c.bot.Chats[message.Chat.ID].SmokeSession.SmokeStarter {
		c.bot.Chats[message.Chat.ID].ActiveSmoke = false
		answer := fmt.Sprintf("@%s завершил перекур!", message.From.UserName)
		c.sendMessage(message.Chat.ID, answer)

		deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, int(c.bot.Chats[message.Chat.ID].SmokeSession.SmokeMessageID))
		c.bot.Bot.Send(deleteMsg)
	} else if c.bot.Chats[message.Chat.ID].ActiveSmoke {
		answer := fmt.Sprintf("@%s, у тебя нет прав!", message.From.UserName)
		c.sendMessage(message.Chat.ID, answer)
	} else {
		c.sendMessage(message.Chat.ID, "Нет активного перекура!")
	}
}

func (c Controller) handleDeleteUser(message *tgbotapi.Message, repo *repository.SQLiteRepository) {
	_, err := repo.GetByID(message.From.ID)
	if err == sql.ErrNoRows {
		logger.Info("Пользователя нет в базе")
		answer := fmt.Sprintf("❌ @%s, вас не существует", message.From.UserName)
		c.sendMessage(message.Chat.ID, answer)
		return
	}

	err = repo.DeleteByID(message.From.ID)
	if err != nil {
		logger.Error("Ошибка удаления пользователя из БД", err)
		return
	}
	answer := fmt.Sprintf("✅ @%s уволен!", message.From.UserName)
	c.sendMessage(message.Chat.ID, answer)
}

func getUsers(users []string) string {
	result := ""
	for i, user := range users {
		if i > 0 {
			result += " "
		}
		result += user
	}
	return result
}

func (c Controller) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	c.bot.Bot.Send(msg)
}

// Регистрация команд бота
func (c Controller) SetCommands() {
	commands := []tgbotapi.BotCommand{
		{
			Command:     "start",
			Description: "Войти в систему",
		},
		{
			Command:     "smoke",
			Description: "Предложить перекур",
		},
		//{
		//	Command:     "status",
		//	Description: "Статус текущего перекура",
		//},
		{
			Command:     "cancel",
			Description: "Отменить перекур",
		},
		{
			Command:     "delete",
			Description: "Удалиться из базы",
		},
		{
			Command:     "help",
			Description: "Помощь",
		},
		//{
		//	Command:     "stats",
		//	Description: "Статистика перекуров",
		//},
	}

	config := tgbotapi.NewSetMyCommands(commands...)
	c.bot.Bot.Request(config)
	//_, err := bot.Request(config)
	//return err
}
