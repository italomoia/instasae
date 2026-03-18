package handler

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/service"
)

type OAuthHandler struct {
	oauthService *service.OAuthService
}

func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthService: oauthService}
}

func (h *OAuthHandler) HandleConnect(w http.ResponseWriter, r *http.Request) {
	if h.oauthService == nil {
		writeErrorPage(w, http.StatusServiceUnavailable, "OAuth não configurado. Entre em contato com o administrador.")
		return
	}

	accountID := r.URL.Query().Get("account_id")
	if _, err := uuid.Parse(accountID); err != nil {
		writeErrorPage(w, http.StatusBadRequest, "ID de conta inválido.")
		return
	}

	exists, err := h.oauthService.AccountExists(r.Context(), accountID)
	if err != nil {
		writeErrorPage(w, http.StatusInternalServerError, "Erro ao verificar conta.")
		return
	}
	if !exists {
		writeErrorPage(w, http.StatusNotFound, "Conta não encontrada.")
		return
	}

	authURL := h.oauthService.GetAuthorizationURL(accountID)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if h.oauthService == nil {
		writeErrorPage(w, http.StatusServiceUnavailable, "OAuth não configurado.")
		return
	}

	// Check if user denied authorization
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		reason := r.URL.Query().Get("error_reason")
		writeErrorPage(w, http.StatusOK, fmt.Sprintf("Autorização negada: %s", reason))
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state") // account_id

	if code == "" || state == "" {
		writeErrorPage(w, http.StatusBadRequest, "Parâmetros ausentes na resposta do Instagram.")
		return
	}

	if err := h.oauthService.HandleCallback(r.Context(), code, state); err != nil {
		writeErrorPage(w, http.StatusInternalServerError, fmt.Sprintf("Erro ao conectar conta: %s", err.Error()))
		return
	}

	http.Redirect(w, r, "/oauth/success", http.StatusFound)
}

func (h *OAuthHandler) HandleSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Conta Conectada</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
  <h1>&#x2705; Conta conectada com sucesso!</h1>
  <p>Sua conta do Instagram foi conectada ao sistema de atendimento.</p>
  <p>Você já pode fechar esta janela.</p>
</body>
</html>`)
}

func writeErrorPage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Erro</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
  <h1>&#x274C; Erro ao conectar</h1>
  <p>%s</p>
  <p>Entre em contato com o suporte.</p>
</body>
</html>`, message)
}
