package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) RegisterRoutes() {
	// Webhooks
	s.router.Route("/webhook", func(r chi.Router) {
		r.Get("/instagram", stubHandler("GET /webhook/instagram"))
		r.Post("/instagram", stubHandler("POST /webhook/instagram"))
		r.Post("/chatwoot", stubHandler("POST /webhook/chatwoot"))
	})

	// Admin API
	s.router.Route("/api/accounts", func(r chi.Router) {
		r.Get("/", stubHandler("GET /api/accounts"))
		r.Post("/", stubHandler("POST /api/accounts"))
		r.Get("/{id}", stubHandler("GET /api/accounts/{id}"))
		r.Put("/{id}", stubHandler("PUT /api/accounts/{id}"))
		r.Delete("/{id}", stubHandler("DELETE /api/accounts/{id}"))
		r.Post("/{id}/refresh-token", stubHandler("POST /api/accounts/{id}/refresh-token"))
		r.Get("/{id}/status", stubHandler("GET /api/accounts/{id}/status"))
	})

	// System
	s.router.Get("/health", stubHandler("GET /health"))
}

func stubHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("stub handler called", "route", name)
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not implemented"})
	}
}
