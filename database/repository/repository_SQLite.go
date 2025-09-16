package repository

import (
	"database/sql"
	"smoke-bot/database/models"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SQLiteRepository реализует интерфейс Repository для SQLite
type SQLiteRepository struct {
	table string
	db    *sql.DB
	mu    sync.RWMutex
}

// NewSQLiteRepository создает новый репозиторий для SQLite
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		table: "users",
		db:    db,
	}
}

// База данных функции
func (r *SQLiteRepository) SaveUser(user *tgbotapi.User, chatID int64) error {
	_, err := r.db.Exec(`
        INSERT OR REPLACE INTO users (chat_id, username)
        VALUES (?, ?)
    `, chatID, user.UserName)
	return err
}

func (r *SQLiteRepository) GetAllUsers() ([]models.User, error) {
	rows, err := r.db.Query(`
        SELECT id, chat_id, username
        FROM users WHERE chat_id != 0
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User

		err := rows.Scan(&user.ID, &user.ChatID, &user.Username)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	return users, nil
}
