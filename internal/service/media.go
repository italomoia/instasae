package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/domain"
)

const maxMediaSize = 25 * 1024 * 1024 // 25MB

type MediaService struct {
	b2         domain.B2Client
	httpClient *http.Client
}

func NewMediaService(b2 domain.B2Client, httpClient *http.Client) *MediaService {
	return &MediaService{b2: b2, httpClient: httpClient}
}

func (s *MediaService) DownloadAndUpload(ctx context.Context, sourceURL string, accountID string, contentType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating download request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxMediaSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("reading media: %w", err)
	}
	if len(data) > maxMediaSize {
		return "", fmt.Errorf("media too large: exceeds 25MB limit")
	}

	ext := ContentTypeToExt(contentType)
	date := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("%s/%s/%s.%s", accountID, date, uuid.New().String(), ext)

	publicURL, err := s.b2.Upload(ctx, key, strings.NewReader(string(data)), contentType)
	if err != nil {
		return "", fmt.Errorf("uploading media: %w", err)
	}

	return publicURL, nil
}

func ContentTypeToExt(contentType string) string {
	ct := strings.ToLower(contentType)
	// Strip parameters like charset
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)

	switch ct {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "video/mp4":
		return "mp4"
	case "audio/ogg", "video/ogg":
		return "ogg"
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "audio/aac":
		return "aac"
	case "audio/mp4", "audio/m4a":
		return "m4a"
	case "video/webm":
		return "webm"
	case "video/avi", "video/x-msvideo":
		return "avi"
	case "video/quicktime":
		return "mov"
	default:
		return "bin"
	}
}
