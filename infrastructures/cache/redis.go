package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("redis: key not found")

// Client is a small, testable surface over Redis.
type Client interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	GetString(ctx context.Context, key string) (string, error)
	GetBytes(ctx context.Context, key string) ([]byte, error)
	Del(ctx context.Context, keys ...string) (int64, error)

	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Exists(ctx context.Context, keys ...string) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)

	SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error
	GetJSON(ctx context.Context, key string, out any) error // ErrNotFound for missing

	Ping(ctx context.Context) error
	Close() error
}

// Config controls Redis connection behavior.
type Config struct {
	Addr     string // "host:port"
	Password string // optional
	DB       int    // database index

	// Timeouts (good starting defaults applied if zero)
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Pooling
	PoolSize     int // total connections
	MinIdleConns int

	// Retries for commands (not connect-level)
	MaxRetries int

	// TLS (optional)
	TLSConfig *tls.Config

	// Optional key prefix (e.g., "app:prod:")
	KeyPrefix string

	// Health check timeout when constructing client (default 2s)
	StartupPingTimeout time.Duration
}

type redisClient struct {
	rdb       *redis.Client
	keyPrefix string
}

func (c *Config) withDefaults() *Config {
	cp := *c
	if cp.DialTimeout == 0 {
		cp.DialTimeout = 5 * time.Second
	}
	if cp.ReadTimeout == 0 {
		cp.ReadTimeout = 3 * time.Second
	}
	if cp.WriteTimeout == 0 {
		cp.WriteTimeout = 3 * time.Second
	}
	if cp.PoolSize == 0 {
		cp.PoolSize = 20
	}
	if cp.MinIdleConns == 0 {
		cp.MinIdleConns = 5
	}
	if cp.MaxRetries == 0 {
		cp.MaxRetries = 2
	}
	if cp.StartupPingTimeout == 0 {
		cp.StartupPingTimeout = 2 * time.Second
	}
	return &cp
}

// New creates a ready-to-use Client and verifies connectivity with PING.
func New(cfg Config) (Client, error) {
	c := cfg.withDefaults()

	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Addr,
		Password:     c.Password,
		DB:           c.DB,
		DialTimeout:  c.DialTimeout,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
		PoolSize:     c.PoolSize,
		MinIdleConns: c.MinIdleConns,
		MaxRetries:   c.MaxRetries,
		TLSConfig:    c.TLSConfig,
	})

	// Health check at startup with a bounded timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.StartupPingTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}

	return &redisClient{rdb: rdb, keyPrefix: c.KeyPrefix}, nil
}

func (c *redisClient) Close() error { return c.rdb.Close() }

func (c *redisClient) key(k string) string {
	if c.keyPrefix == "" {
		return k
	}
	return c.keyPrefix + k
}

func (c *redisClient) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *redisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.key(key), value, ttl).Err()
}

func (c *redisClient) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, c.key(key), value, ttl).Result()
}

func (c *redisClient) GetString(ctx context.Context, key string) (string, error) {
	v, err := c.rdb.Get(ctx, c.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return v, err
}

func (c *redisClient) GetBytes(ctx context.Context, key string) ([]byte, error) {
	v, err := c.rdb.Get(ctx, c.key(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	return v, err
}

func (c *redisClient) Del(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	ks := make([]string, len(keys))
	for i, k := range keys {
		ks[i] = c.key(k)
	}
	return c.rdb.Del(ctx, ks...).Result()
}

func (c *redisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	ks := make([]string, len(keys))
	for i, k := range keys {
		ks[i] = c.key(k)
	}
	return c.rdb.Exists(ctx, ks...).Result()
}

func (c *redisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, c.key(key)).Result()
}

func (c *redisClient) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.rdb.Expire(ctx, c.key(key), ttl).Result()
}

func (c *redisClient) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.key(key), b, ttl).Err()
}

func (c *redisClient) GetJSON(ctx context.Context, key string, out any) error {
	b, err := c.GetBytes(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
