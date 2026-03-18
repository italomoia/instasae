package server

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/italomoia/instasae/internal/handler"
	"github.com/italomoia/instasae/internal/middleware"
)

type Handlers struct {
	WebhookInstagram *handler.WebhookInstagramHandler
	WebhookChatwoot  *handler.WebhookChatwootHandler
	AdminAccounts    *handler.AdminAccountsHandler
	Health           *handler.HealthHandler
	OAuth            *handler.OAuthHandler
}

func (s *Server) RegisterRoutes(h Handlers, logger *slog.Logger) {
	s.router.Use(middleware.RequestLogging(logger))

	// OAuth flow (public — client-facing)
	s.router.Get("/connect", h.OAuth.HandleConnect)
	s.router.Get("/oauth/callback", h.OAuth.HandleCallback)
	s.router.Get("/oauth/success", h.OAuth.HandleSuccess)

	// Webhooks
	s.router.Route("/webhook", func(r chi.Router) {
		// GET verification — no signature check
		r.Get("/instagram", h.WebhookInstagram.HandleVerification)

		// POST webhook — signature middleware
		r.With(middleware.SignatureValidation(h.WebhookInstagram.IGService())).
			Post("/instagram", h.WebhookInstagram.HandleWebhook)

		// Chatwoot callback — no auth (Chatwoot does not send API keys)
		r.Post("/chatwoot", h.WebhookChatwoot.HandleCallback)
	})

	// Admin API — all routes require API key
	s.router.Route("/api/accounts", func(r chi.Router) {
		r.Use(middleware.APIKeyAuth(s.cfg.AdminAPIKey))
		r.Get("/", h.AdminAccounts.HandleList)
		r.Post("/", h.AdminAccounts.HandleCreate)
		r.Get("/{id}", h.AdminAccounts.HandleGetByID)
		r.Put("/{id}", h.AdminAccounts.HandleUpdate)
		r.Delete("/{id}", h.AdminAccounts.HandleDelete)
	})

	// System — no auth
	s.router.Get("/health", h.Health.HandleHealth)
}
