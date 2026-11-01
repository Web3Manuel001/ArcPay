package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(rawURL string) (*Client, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("REDIS_URL cannot be empty")
	}

	opt, err := redis.ParseURL(rawURL)
	if err != nil {
		// Fallback for standard host:port
		opt = &redis.Options{
			Addr: rawURL,
		}
		if strings.HasPrefix(rawURL, "rediss://") {
			opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
	}

	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis Cloud: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) AcquireLock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("lock:%s", key)
	return c.rdb.SetNX(ctx, lockKey, "1", expiration).Result()
}

func (c *Client) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("lock:%s", key)
	return c.rdb.Del(ctx, lockKey).Err()
}
