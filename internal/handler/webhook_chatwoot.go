package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

type WebhookChatwootHandler struct {
	cwService *service.ChatwootService
}

func NewWebhookChatwootHandler(cwService *service.ChatwootService) *WebhookChatwootHandler {
	return &WebhookChatwootHandler{cwService: cwService}
}

func (h *WebhookChatwootHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var payload model.CWWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Warn("failed to decode chatwoot webhook payload", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	if err := h.cwService.ProcessCallback(r.Context(), payload); err != nil {
		slog.Error("chatwoot callback processing failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "processing failed"})
		return
	}

	w.WriteHeader(http.StatusOK)
}
