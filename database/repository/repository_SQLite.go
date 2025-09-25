package repository

import (
	"database/sql"
	"fmt"
	"smoke-bot/models"
	"sync"
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
func (r *SQLiteRepository) SaveUser(user_id int64, user string, chatID int64) error {
	_, err := r.db.Exec(`
        INSERT OR REPLACE INTO users (user_id, chat_id, username)
        VALUES (?, ?, ?)
    `, user_id, chatID, user)
	return err
}

func (r *SQLiteRepository) GetAllUsers() ([]models.User, error) {
	rows, err := r.db.Query(`
        SELECT user_id, chat_id, username
        FROM users WHERE chat_id != 0
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User

		err := rows.Scan(&user.UserID, &user.ChatID, &user.Username)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	return users, nil
}

func (r *SQLiteRepository) GetByID(ID int64) (*models.User, error) {
	var user models.User

	err := r.db.QueryRow(`
        SELECT user_id, chat_id, username
        FROM users WHERE user_id = ?
    `, ID).Scan(&user.UserID, &user.ChatID, &user.Username)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("ошибка получения пользователя: %v", err)
	}

	return &user, nil
}

func (r *SQLiteRepository) DeleteByID(ID int64) error {
	result, err := r.db.Exec("DELETE FROM users WHERE user_id = ?", ID)
	if err != nil {
		return fmt.Errorf("ошибка удаления пользователя: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка получения количества удаленных строк: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("пользователь с ID %d не найден", ID)
	}

	return nil
}
