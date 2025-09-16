package database

import (
	"database/sql"
	"smoke-bot/logger"
	"sync"
)

type SQLiteDB struct {
	db *sql.DB
}

var (
	sqliteInstance  *SQLiteDB
	sqliteOnce      sync.Once
	sqliteInitError error
)

func InitSQLite(dbPath string) (*SQLiteDB, error) {
	sqliteOnce.Do(func() {
		logger.Info("Инициализация SQLite: " + dbPath)

		// Открываем соединение с SQLite
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			sqliteInitError = err
			logger.Error("Ошибка открытия БД", err)
			return
		}

		// Проверяем соединение
		if err := db.Ping(); err != nil {
			sqliteInitError = err
			logger.Error("Ошибка подключения к БД", err)
			return
		}

		if err := createTables(db); err != nil {
			sqliteInitError = err
			logger.Error("Ошибка создания таблицы БД", err)
			return
		}

		sqliteInstance = &SQLiteDB{db: db}
		logger.Success("SQLite инициализирован: " + dbPath)
	})

	return sqliteInstance, sqliteInitError
}

// Создание таблиц
func createTables(db *sql.DB) error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            chat_id INTEGER UNIQUE NOT NULL,
            username TEXT,
        )
    `)
	if err != nil {
		return err
	}

	logger.Success("Таблицы базы данных созданы/проверены")
	return err
}

func (s *SQLiteDB) Close() error {
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		logger.Info("Соединение с SQLite закрыто")
		return err
	}
	return nil
}

func (s *SQLiteDB) GetDB() *sql.DB {
	return s.db
}
