package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/italomoia/instasae/internal/config"
)

type Server struct {
	httpServer *http.Server
	router     chi.Router
	cfg        *config.Config
}

func NewServer(cfg *config.Config, h Handlers, logger *slog.Logger) *Server {
	r := chi.NewRouter()

	s := &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      r,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		router: r,
		cfg:    cfg,
	}

	s.RegisterRoutes(h, logger)

	return s
}

func (s *Server) Start() error {
	slog.Info("server starting", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("server shutting down")
	return s.httpServer.Shutdown(ctx)
}
