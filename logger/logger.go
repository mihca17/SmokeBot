package logger

import (
	"io"
	"log"
	"os"
	"sync"
)

var (
	successLogger *log.Logger
	errorLogger   *log.Logger
	infoLogger    *log.Logger
	logFile       *os.File
	once          sync.Once
)

// Init инициализирует логгер с выводом в консоль и файл
func Init(logFilename string) error {
	var initErr error
	once.Do(func() {

		file, err := os.OpenFile(logFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			initErr = err
			return
		}
		multiWriter := io.MultiWriter(os.Stdout, file)

		successLogger = log.New(multiWriter, "✅ ", log.Ldate|log.Ltime)
		errorLogger = log.New(multiWriter, "❌ ", log.Ldate|log.Ltime)
		infoLogger = log.New(multiWriter, "ℹ️ ", log.Ldate|log.Ltime)
	})
	return initErr
}

// Close закрывает файл логов
func Close() error {
	if logFile != nil {
		return logFile.Close()
	}
	return nil
}

// Success логирует успешные операции
func Success(message string) {
	successLogger.Println(message)
}

// Error логирует ошибки
func Error(message string, err error) {
	if err != nil {
		errorLogger.Printf("%s: %v", message, err)
	} else {
		errorLogger.Println(message)
	}
}

// Info логирует информационные сообщения
func Info(message string) {
	infoLogger.Println(message)
}

// Fatal логирует фатальную ошибку и завершает программу
func Fatal(message string, err error) {
	if err != nil {
		errorLogger.Fatalf("%s: %v", message, err)
	} else {
		errorLogger.Fatal(message)
	}
}
