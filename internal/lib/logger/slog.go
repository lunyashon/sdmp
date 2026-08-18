package sl

import (
	"io"
	"log"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

func ExecLog(path string) *slog.Logger {

	var (
		level  slog.Level
		writer io.Writer
	)

	// Если путь пустой, используем значение по умолчанию
	if path == "" {
		path = "./logs"
	}

	level = slog.LevelInfo
	writer = io.MultiWriter(
		os.Stdout,
		createFileWriter(path),
	)

	return slog.New(
		slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level}),
	)
}

func createFileWriter(path string) io.Writer {
	// Создаем директорию для логов, если её нет
	if err := os.MkdirAll(path, 0755); err != nil {
		log.Printf("Failed to create log directory: %v\n", err)
		panic(err)
	}

	logFile := path + "/info.log"
	log.Printf("Logger initialized. Log file: %s\n", logFile)

	return &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    25,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}
}

func getConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}
