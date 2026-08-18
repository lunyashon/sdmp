package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

func New(port string, handler http.Handler, logger *slog.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%s", port),
			Handler: handler,
			// Безопасные сетевые таймауты
			ReadHeaderTimeout: 5 * time.Second,  // Защита от Slowloris (чтение заголовков)
			ReadTimeout:       10 * time.Second, // Макс. время чтения всего запроса (body)
			WriteTimeout:      10 * time.Second, // Макс. время записи ответа
			IdleTimeout:       30 * time.Second, // Время жизни простаивающего Keep-Alive соединения
		},
		logger: logger,
	}
}

func (s *Server) Run() error {
	s.logger.Info("server starting", "addr", s.httpServer.Addr)

	err := s.httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down http server...")
	return s.httpServer.Shutdown(ctx)
}
