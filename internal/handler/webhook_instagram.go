package handler

import (
	"context"
	"encoding/json"
	"io"
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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Warn("failed to read webhook body", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	slog.Debug("webhook body received", "length", len(body))

	var payload model.IGWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		slog.Warn("failed to decode instagram webhook payload", "error", err, "body_length", len(body), "body_preview", preview)
		w.WriteHeader(http.StatusOK)
		return
	}

	slog.Debug("webhook payload parsed", "object", payload.Object, "entries", len(payload.Entry))

	// BR-WEBHOOK-01: Respond 200 IMMEDIATELY, process async
	w.WriteHeader(http.StatusOK)

	go h.igService.ProcessWebhook(context.Background(), payload)
}
