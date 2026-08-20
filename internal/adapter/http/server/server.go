package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/lunyashon/sdmp/internal/lib/config"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

func New(
	port string,
	handler http.Handler,
	logger *slog.Logger,
	config *config.Config,
) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%s", port),
			Handler: handler,
			// Безопасные сетевые таймауты
			ReadHeaderTimeout: time.Duration(config.ReadHeaderTimeout) * time.Second, // Защита от Slowloris (чтение заголовков)
			ReadTimeout:       time.Duration(config.ReadTimeout) * time.Second,       // Макс. время чтения всего запроса (body)
			WriteTimeout:      time.Duration(config.WriteTimeout) * time.Second,      // Макс. время записи ответа
			IdleTimeout:       time.Duration(config.IdleTimeout) * time.Second,       // Время жизни простаивающего Keep-Alive соединения
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
