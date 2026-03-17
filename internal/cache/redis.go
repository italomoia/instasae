package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

var _ domain.Cache = (*RedisCache)(nil)

const keyPrefix = "instasae:"

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (c *RedisCache) SetDedup(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
	ok, err := c.client.SetNX(ctx, keyPrefix+"dedup:"+messageID, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("set dedup: %w", err)
	}
	return ok, nil
}

func (c *RedisCache) GetAccount(ctx context.Context, igPageID string) (*model.Account, error) {
	data, err := c.client.Get(ctx, keyPrefix+"account:"+igPageID).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get account cache: %w", err)
	}

	var acc model.Account
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&acc); err != nil {
		return nil, fmt.Errorf("decode account cache: %w", err)
	}
	return &acc, nil
}

func (c *RedisCache) SetAccount(ctx context.Context, igPageID string, account *model.Account, ttl time.Duration) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(account); err != nil {
		return fmt.Errorf("encode account cache: %w", err)
	}
	data := buf.Bytes()

	if err := c.client.Set(ctx, keyPrefix+"account:"+igPageID, data, ttl).Err(); err != nil {
		return fmt.Errorf("set account cache: %w", err)
	}
	return nil
}

func (c *RedisCache) DeleteAccount(ctx context.Context, igPageID string) error {
	if err := c.client.Del(ctx, keyPrefix+"account:"+igPageID).Err(); err != nil {
		return fmt.Errorf("delete account cache: %w", err)
	}
	return nil
}
