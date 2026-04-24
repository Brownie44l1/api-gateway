package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

type Response struct {
	Body       string            `json:"body"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Timestamp  time.Time         `json:"timestamp"`
}

func New(addr, password string, ttl time.Duration) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	return &Cache{
		client: client,
		ttl:    ttl,
	}
}

func (c *Cache) Close() error {
	return c.client.Close()
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Cache) key(path, query string) string {
	if query != "" {
		return fmt.Sprintf("cache:%s?%s", path, query)
	}
	return fmt.Sprintf("cache:%s", path)
}

func (c *Cache) Get(ctx context.Context, path, query string) (*Response, bool) {
	key := c.key(path, query)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false
	}

	return &resp, true
}

func (c *Cache) Set(ctx context.Context, path, query string, resp *Response) error {
	key := c.key(path, query)
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, c.ttl).Err()
}