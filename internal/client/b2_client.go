package client

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/italomoia/instasae/internal/domain"
)

var _ domain.B2Client = (*B2Storage)(nil)

type B2Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	KeyID          string
	ApplicationKey string
	PublicURL      string
	Prefix         string
}

type B2Storage struct {
	client    *s3.Client
	bucket    string
	publicURL string
	prefix    string
}

func NewB2Storage(cfg B2Config) *B2Storage {
	client := s3.New(s3.Options{
		BaseEndpoint: &cfg.Endpoint,
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.KeyID, cfg.ApplicationKey, ""),
		UsePathStyle: true,
	})

	return &B2Storage{
		client:    client,
		bucket:    cfg.Bucket,
		publicURL: cfg.PublicURL,
		prefix:    cfg.Prefix,
	}
}

func (b *B2Storage) Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	fullKey := b.prefix + "/" + key

	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &b.bucket,
		Key:         &fullKey,
		Body:        data,
		ContentType: &contentType,
	})
	if err != nil {
		return "", fmt.Errorf("uploading to B2: %w", err)
	}

	publicURL := fmt.Sprintf("%s/%s", b.publicURL, fullKey)
	return publicURL, nil
}

func (b *B2Storage) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", b.publicURL, b.prefix, key)
}
