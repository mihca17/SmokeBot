package main

import (
	"fmt"
	"log"
	"smoke-bot/config"
	"smoke-bot/database/database"
	"smoke-bot/database/repository"
	"smoke-bot/logger"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "modernc.org/sqlite"
)

var (
	bot         *tgbotapi.BotAPI
	activeSmoke bool
	mu          sync.Mutex
	//userStart         = make(map[int64]bool)
	smokeStarter      int64
	smokeMessageID    int64
	joinedUsers       []string
	originalSmokeText string
	allowedChats      = map[int64]bool{
		-4845216092: true,
	}
)

func main() {
	config := config.DefaultConfig()

	err := logger.Init(config.LogFile)
	if err != nil {
		log.Fatalf("Не удалось инициализировать логгер: %v", err)
	}
	defer logger.Close()

	// Инициализация базы данных
	db, err := database.InitSQLite(config.DBPath)
	if err != nil {
		logger.Fatal("Ошибка инициализации БД", err)
	}
	defer db.Close()

	repo := repository.NewSQLiteRepository(db.GetDB())

	//var err error
	bot, err := tgbotapi.NewBotAPI("8304451768:AAEyfAUAWL2jNgDQI-MfKVHObe71BBtAJ98")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)
	setCommands()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			if isChatAllowed(update.CallbackQuery.Message.Chat.ID) {
				handleCallback(update.CallbackQuery)
			}
			continue
		}

		if update.Message == nil {
			continue
		}

		if !isChatAllowed(update.Message.Chat.ID) {
			continue
		}

		if isOldMessage(update.Message) {
			log.Printf("Пропускаем старое сообщение от %s: %s",
				update.Message.From.UserName, update.Message.Text)
			continue
		}

		// Работает только в групповых чатах
		if !update.Message.Chat.IsGroup() && !update.Message.Chat.IsSuperGroup() {
			continue
		}

		//if update.Message.IsCommand() {
		switch update.Message.Command() {
		case "start":
			handleConsent(update.Message, repo)
		case "smoke":
			handleSmoke(update.Message, repo)
		case "cancel":
			handleEndSmoke(update.Message)
		case "help":
			sendMessage(update.Message.Chat.ID,
				"Поддерживаемые команды:\n\n"+
					"/start - Присоединиться к банде курителей\n"+
					"/smoke - Предложить всем перекур\n"+
					"/cancel - Отменить активный перекур\n"+
					"/help - Помощь")
			//case "yes", "no":
			//	handleResponse(update.Message)
			//default:
			//	sendMessage(update.Message.Chat.ID, fmt.Sprintf("@%s, твоя команда не поддерживается, прожми /help", update.Message.From.UserName))
		}
		//} else {
		//	sendMessage(update.Message.Chat.ID, fmt.Sprintf("@%s, бот принимает только команды", update.Message.From.UserName))
		//}

	}
}

func isOldMessage(message *tgbotapi.Message) bool {
	messageTime := time.Unix(int64(message.Date), 0)
	botStartTime := time.Now().Add(-5 * time.Minute) // Допускаем небольшую погрешность

	return messageTime.Before(botStartTime)
}

func isChatAllowed(chatID int64) bool {
	//if len(allowedChats) == 0 {
	//	return true
	//}

	_, exists := allowedChats[chatID]
	return exists
}

func handleConsent(message *tgbotapi.Message, repo *repository.SQLiteRepository) {
	repo.SaveUser(message.From.UserName, message.Chat.ID)
	answer := fmt.Sprintf("✅ @%s теперь с нами!", message.From.UserName)
	sendMessage(message.Chat.ID, answer)
}

func handleSmoke(message *tgbotapi.Message, repo *repository.SQLiteRepository) {
	mu.Lock()
	defer mu.Unlock()

	if activeSmoke {
		answer := fmt.Sprintf("❌ @%s, уже кто-то предложил перекур!", message.From.UserName)
		sendMessage(message.Chat.ID, answer)
		return
	}

	activeSmoke = true
	smokeStarter = message.From.ID
	//joinedUsers = []string{}

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
		sendMessage(message.Chat.ID, "❌ Пока никто не добавился ;(")
		activeSmoke = false
		return
	}

	originalSmokeText = fmt.Sprintf("🚬 @%s предлагает перекур! \nПриглашённые джентльмены: %s",
		message.From.UserName, getUsers(users))

	msg := tgbotapi.NewMessage(message.Chat.ID, originalSmokeText)

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да", "yes"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Нет", "no"),
		),
	)
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Ошибка отправки сообщения с кнопками: %v", err)
		activeSmoke = false
		return
	}
	smokeMessageID = int64(sentMsg.MessageID)
	//sendMessage(message.Chat.ID, msg.Text)
}

func handleCallback(callback *tgbotapi.CallbackQuery) {
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackConfig)

	if callback.Data == "yes" || callback.Data == "no" {
		handleButtonResponse(callback)
	}
}

func handleButtonResponse(callback *tgbotapi.CallbackQuery) {
	mu.Lock()
	defer mu.Unlock()

	if !activeSmoke {
		return
	}

	if callback.Data == "yes" {
		userName := fmt.Sprintf("@%s", callback.From.UserName)

		for _, user := range joinedUsers {
			if user == userName {
				return
			}
		}
		joinedUsers = append(joinedUsers, userName)

		// Редактируем сообщение с обновленным списком
		editMessage(callback.Message.Chat.ID, smokeMessageID)
	}
	// Для "no" ничего не делаем
}

func editMessage(chatID int64, messageID int64) {
	newText := fmt.Sprintf("%s\n\nПрисоединились: %s",
		originalSmokeText,
		getUsers(joinedUsers))

	if len(joinedUsers) == 0 {
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
	bot.Send(edit)
}

func handleEndSmoke(message *tgbotapi.Message) {
	mu.Lock()
	defer mu.Unlock()

	if activeSmoke && message.From.ID == smokeStarter {
		activeSmoke = false
		answer := fmt.Sprintf("@%s завершил перекур!", message.From.UserName)
		sendMessage(message.Chat.ID, answer)

		deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, int(smokeMessageID))
		bot.Send(deleteMsg)
	} else if activeSmoke {
		answer := fmt.Sprintf("@%s, у тебя нет прав!", message.From.UserName)
		sendMessage(message.Chat.ID, answer)
	} else {
		sendMessage(message.Chat.ID, "Нет активного перекура!")
	}
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

func sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}

// Регистрация команд бота
func setCommands() {
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
			Command:     "help",
			Description: "Помощь",
		},
		//{
		//	Command:     "stats",
		//	Description: "Статистика перекуров",
		//},
	}

	config := tgbotapi.NewSetMyCommands(commands...)
	bot.Request(config)
	//_, err := bot.Request(config)
	//return err
}
