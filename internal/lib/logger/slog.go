package sl

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

func ExecLog(path, level string, maxSize, maxAge, maxBackups int) *slog.Logger {
	if path == "" {
		path = "./logs/app.log"
	}
	if maxSize <= 0 {
		maxSize = 10
	}
	if maxAge <= 0 {
		maxAge = 7
	}
	if maxBackups <= 0 {
		maxBackups = 10
	}

	writer := io.MultiWriter(os.Stdout, createFileWriter(path, maxSize, maxAge, maxBackups))

	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: ParseLevel(level)}))
}

func createFileWriter(path string, maxSize, maxAge, maxBackups int) io.Writer {
	dir := path
	file := filepath.Join(path, "info.log")
	if filepath.Ext(path) != "" {
		dir = filepath.Dir(path)
		file = path
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create log directory, fallback to stdout", "error", err, "path", dir)
		return os.Stdout
	}

	return &lumberjack.Logger{
		Filename:   file,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   true,
		LocalTime:  true,
	}
}

func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
