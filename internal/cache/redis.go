package cache

import (
	"context"
	"encoding/json"
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
	if err := json.Unmarshal(data, &acc); err != nil {
		return nil, fmt.Errorf("unmarshal account cache: %w", err)
	}
	return &acc, nil
}

func (c *RedisCache) SetAccount(ctx context.Context, igPageID string, account *model.Account, ttl time.Duration) error {
	data, err := json.Marshal(account)
	if err != nil {
		return fmt.Errorf("marshal account cache: %w", err)
	}

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
