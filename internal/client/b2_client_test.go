package client_test

import (
	"testing"

	"github.com/italomoia/instasae/internal/client"
)

func TestB2PublicURL(t *testing.T) {
	b2 := client.NewB2Storage(client.B2Config{
		Endpoint:       "https://s3.us-west-004.backblazeb2.com",
		Region:         "us-west-004",
		Bucket:         "my-bucket",
		KeyID:          "key",
		ApplicationKey: "secret",
		PublicURL:      "https://f004.backblazeb2.com/file/my-bucket",
		Prefix:         "instasae",
	})

	got := b2.PublicURL("abc123/2026-03-16/img.jpg")
	want := "https://f004.backblazeb2.com/file/my-bucket/instasae/abc123/2026-03-16/img.jpg"
	if got != want {
		t.Errorf("PublicURL = %q, want %q", got, want)
	}
}

func TestB2UploadURLFormat(t *testing.T) {
	// Verify that Upload would construct the correct public URL.
	// Actual S3 upload is tested in integration tests.
	// This test validates the URL construction logic matches the expected pattern:
	// {B2_PUBLIC_URL}/{B2_PREFIX}/{key}
	b2 := client.NewB2Storage(client.B2Config{
		Endpoint:       "https://s3.example.com",
		Region:         "us-west-004",
		Bucket:         "test-bucket",
		KeyID:          "key",
		ApplicationKey: "secret",
		PublicURL:      "https://cdn.example.com/file/test-bucket",
		Prefix:         "instasae",
	})

	got := b2.PublicURL("acct-id/2026-03-16/550e8400.jpg")
	want := "https://cdn.example.com/file/test-bucket/instasae/acct-id/2026-03-16/550e8400.jpg"
	if got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}
