package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

type WebhookInstagramHandler struct {
	igService        *service.InstagramService
	verifyToken      string
}

func NewWebhookInstagramHandler(igService *service.InstagramService, verifyToken string) *WebhookInstagramHandler {
	return &WebhookInstagramHandler{
		igService:   igService,
		verifyToken: verifyToken,
	}
}

func (h *WebhookInstagramHandler) IGService() *service.InstagramService {
	return h.igService
}

func (h *WebhookInstagramHandler) HandleVerification(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == h.verifyToken {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}

	w.WriteHeader(http.StatusForbidden)
}

func (h *WebhookInstagramHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload model.IGWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Warn("failed to decode instagram webhook payload", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// BR-WEBHOOK-01: Respond 200 IMMEDIATELY, process async
	w.WriteHeader(http.StatusOK)

	go h.igService.ProcessWebhook(context.Background(), payload)
}
