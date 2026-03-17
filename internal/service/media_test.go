package service_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/italomoia/instasae/internal/service"
)

func TestDownloadAndUpload_Success(t *testing.T) {
	mediaData := []byte("fake-image-data")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(mediaData)
	}))
	defer srv.Close()

	var uploadedKey string
	var uploadedCT string
	b2 := &mockB2Client{
		UploadFn: func(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
			uploadedKey = key
			uploadedCT = contentType
			return "https://cdn.example.com/" + key, nil
		},
	}

	svc := service.NewMediaService(b2, srv.Client())
	url, err := svc.DownloadAndUpload(context.Background(), srv.URL+"/img.jpg", "acct-123", "image/jpeg")
	if err != nil {
		t.Fatalf("DownloadAndUpload: %v", err)
	}

	if !strings.HasPrefix(url, "https://cdn.example.com/acct-123/") {
		t.Errorf("URL = %q, should start with acct prefix", url)
	}
	if !strings.HasSuffix(uploadedKey, ".jpg") {
		t.Errorf("key = %q, should end with .jpg", uploadedKey)
	}
	if uploadedCT != "image/jpeg" {
		t.Errorf("contentType = %q, want image/jpeg", uploadedCT)
	}
}

func TestDownloadAndUpload_DownloadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b2 := &mockB2Client{
		UploadFn: func(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
			t.Error("Upload should not be called on download failure")
			return "", nil
		},
	}

	svc := service.NewMediaService(b2, srv.Client())
	_, err := svc.DownloadAndUpload(context.Background(), srv.URL+"/fail", "acct-123", "image/jpeg")
	if err == nil {
		t.Error("should return error on download failure")
	}
}

func TestDownloadAndUpload_UploadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	b2 := &mockB2Client{
		UploadFn: func(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
			return "", fmt.Errorf("b2 upload failed")
		},
	}

	svc := service.NewMediaService(b2, srv.Client())
	_, err := svc.DownloadAndUpload(context.Background(), srv.URL, "acct-123", "image/png")
	if err == nil {
		t.Error("should return error on upload failure")
	}
}

func TestDownloadAndUpload_TooLarge(t *testing.T) {
	// Generate data slightly over 25MB
	bigData := bytes.Repeat([]byte("x"), 25*1024*1024+100)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bigData)
	}))
	defer srv.Close()

	b2 := &mockB2Client{
		UploadFn: func(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
			t.Error("Upload should not be called for oversized media")
			return "", nil
		},
	}

	svc := service.NewMediaService(b2, srv.Client())
	_, err := svc.DownloadAndUpload(context.Background(), srv.URL, "acct-123", "video/mp4")
	if err == nil {
		t.Error("should return error for oversized media")
	}
	if !strings.Contains(err.Error(), "25MB") {
		t.Errorf("error = %q, should mention 25MB", err.Error())
	}
}

func TestContentTypeToExtension(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{"image/jpeg", "jpg"},
		{"image/png", "png"},
		{"image/gif", "gif"},
		{"image/webp", "webp"},
		{"video/mp4", "mp4"},
		{"audio/ogg", "ogg"},
		{"audio/wav", "wav"},
		{"audio/aac", "aac"},
		{"audio/m4a", "m4a"},
		{"video/webm", "webm"},
		{"video/quicktime", "mov"},
		{"application/octet-stream", "bin"},
		{"image/jpeg; charset=utf-8", "jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := service.ContentTypeToExt(tt.contentType)
			if got != tt.want {
				t.Errorf("ContentTypeToExt(%q) = %q, want %q", tt.contentType, got, tt.want)
			}
		})
	}
}
