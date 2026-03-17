package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
)

type SignatureValidator interface {
	ValidateSignature(body []byte, signature string) bool
}

func SignatureValidation(validator SignatureValidator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				slog.Warn("failed to read request body for signature validation")
				w.WriteHeader(http.StatusOK)
				return
			}
			r.Body.Close()

			signature := r.Header.Get("X-Hub-Signature-256")
			if !validator.ValidateSignature(body, signature) {
				slog.Warn("invalid webhook signature", "path", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				return
			}

			slog.Debug("webhook signature valid", "body_length", len(body))
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}
