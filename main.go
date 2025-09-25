package main

import (
	"log"
	"smoke-bot/config"
	"smoke-bot/database/database"
	"smoke-bot/database/repository"
	"smoke-bot/logger"
	"smoke-bot/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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

	var bot models.Bot
	bot.Chats = make(map[int64]*models.Chat)
	bot.Bot, err = tgbotapi.NewBotAPI("8304451768:AAEyfAUAWL2jNgDQI-MfKVHObe71BBtAJ98")
	if err != nil {
		log.Panic(err)
	}

	bot.Bot.Debug = true
	log.Printf("Authorized on account %s", bot.Bot.Self.UserName)

	controller := NewController(&bot)
	controller.SetCommands()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.Bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			if isChatAllowed(update.CallbackQuery.Message.Chat.ID) {
				controller.handleCallback(update.CallbackQuery)
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

		if _, exists := bot.Chats[update.Message.Chat.ID]; !exists {
			smokeSession := &models.SmokeSession{}
			chat := &models.Chat{
				ID:           update.Message.Chat.ID,
				ActiveSmoke:  false,
				SmokeSession: smokeSession,
			}

			bot.Chats[update.Message.Chat.ID] = chat
		}

		switch update.Message.Command() {
		case "start":
			controller.handleConsent(update.Message, repo)
		case "smoke":
			controller.handleSmoke(update.Message, repo)
		case "cancel":
			controller.handleEndSmoke(update.Message)
		case "delete":
			controller.handleDeleteUser(update.Message, repo)
		case "help":
			controller.sendMessage(update.Message.Chat.ID,
				"Поддерживаемые команды:\n\n"+
					"/start - Присоединиться к банде курителей\n"+
					"/smoke - Предложить всем перекур\n"+
					"/cancel - Отменить активный перекур\n"+
					"/delete - Удалиться из базы\n"+
					"/help - Помощь")
		}
	}
}
