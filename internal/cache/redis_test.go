package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/italomoia/instasae/internal/cache"
	"github.com/italomoia/instasae/internal/model"
)

func setupRedis(t *testing.T) *cache.RedisCache {
	t.Helper()

	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		url = "redis://localhost:6382/1"
	}

	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}

	client := redis.NewClient(opts)
	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("redis unavailable, skipping")
	}

	client.FlushDB(ctx)

	t.Cleanup(func() {
		client.FlushDB(ctx)
		client.Close()
	})

	return cache.NewRedisCache(client)
}

func TestSetDedup_FirstTime(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	ok, err := rc.SetDedup(ctx, "msg_001", 5*time.Minute)
	if err != nil {
		t.Fatalf("SetDedup: %v", err)
	}
	if !ok {
		t.Error("first SetDedup should return true")
	}
}

func TestSetDedup_Duplicate(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	_, _ = rc.SetDedup(ctx, "msg_dup", 5*time.Minute)

	ok, err := rc.SetDedup(ctx, "msg_dup", 5*time.Minute)
	if err != nil {
		t.Fatalf("SetDedup: %v", err)
	}
	if ok {
		t.Error("second SetDedup with same ID should return false")
	}
}

func TestSetDedup_DifferentIDs(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	ok1, err := rc.SetDedup(ctx, "msg_a", 5*time.Minute)
	if err != nil {
		t.Fatalf("SetDedup a: %v", err)
	}
	ok2, err := rc.SetDedup(ctx, "msg_b", 5*time.Minute)
	if err != nil {
		t.Fatalf("SetDedup b: %v", err)
	}

	if !ok1 || !ok2 {
		t.Errorf("different IDs should both return true, got ok1=%v ok2=%v", ok1, ok2)
	}
}

func TestSetDedup_TTLExpires(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	_, _ = rc.SetDedup(ctx, "msg_ttl", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	ok, err := rc.SetDedup(ctx, "msg_ttl", 5*time.Minute)
	if err != nil {
		t.Fatalf("SetDedup after TTL: %v", err)
	}
	if !ok {
		t.Error("SetDedup should return true after TTL expired")
	}
}

func TestGetAccount_Miss(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	acc, err := rc.GetAccount(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc != nil {
		t.Error("cache miss should return nil")
	}
}

func TestSetGetAccount(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	want := &model.Account{
		ID:                 uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		IGPageID:           "17841451529395669",
		IGPageName:         "Test Store",
		IGAccessToken:      "ig_token_secret",
		ChatwootBaseURL:    "https://chat.example.com",
		ChatwootAccountID:  1,
		ChatwootInboxID:    5,
		ChatwootAPIToken:   "cw_token_secret",
		WebhookVerifyToken: "verify_secret",
		IsActive:           true,
	}

	if err := rc.SetAccount(ctx, want.IGPageID, want, 5*time.Minute); err != nil {
		t.Fatalf("SetAccount: %v", err)
	}

	got, err := rc.GetAccount(ctx, want.IGPageID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got == nil {
		t.Fatal("GetAccount returned nil")
	}
	if got.ID != want.ID || got.IGPageID != want.IGPageID || got.IGPageName != want.IGPageName {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got.IGAccessToken != want.IGAccessToken {
		t.Errorf("IGAccessToken = %q, want %q", got.IGAccessToken, want.IGAccessToken)
	}
	if got.ChatwootAPIToken != want.ChatwootAPIToken {
		t.Errorf("ChatwootAPIToken = %q, want %q", got.ChatwootAPIToken, want.ChatwootAPIToken)
	}
	if got.WebhookVerifyToken != want.WebhookVerifyToken {
		t.Errorf("WebhookVerifyToken = %q, want %q", got.WebhookVerifyToken, want.WebhookVerifyToken)
	}
}

func TestDeleteAccount(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	acc := &model.Account{
		ID:       uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		IGPageID: "page_del",
	}

	_ = rc.SetAccount(ctx, acc.IGPageID, acc, 5*time.Minute)

	if err := rc.DeleteAccount(ctx, acc.IGPageID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	got, err := rc.GetAccount(ctx, acc.IGPageID)
	if err != nil {
		t.Fatalf("GetAccount after delete: %v", err)
	}
	if got != nil {
		t.Error("GetAccount after delete should return nil")
	}
}

func TestGetAccount_TTLExpires(t *testing.T) {
	rc := setupRedis(t)
	ctx := context.Background()

	acc := &model.Account{
		ID:       uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		IGPageID: "page_ttl",
	}

	_ = rc.SetAccount(ctx, acc.IGPageID, acc, 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	got, err := rc.GetAccount(ctx, acc.IGPageID)
	if err != nil {
		t.Fatalf("GetAccount after TTL: %v", err)
	}
	if got != nil {
		t.Error("GetAccount after TTL should return nil")
	}
}
