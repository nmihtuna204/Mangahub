// Package cache - Redis Cache Interface and Implementation
// Caching layer cho MangaHub với Redis
// Chức năng:
//   - Cache external API responses
//   - Session storage
//   - Rate limiting counters
//   - Real-time data caching
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"mangahub/pkg/config"
)

// Cache interface defines caching operations
type Cache interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key string) (string, error)

	// Set stores a value with optional TTL
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a key
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)

	// GetTTL returns remaining TTL for a key
	GetTTL(ctx context.Context, key string) (time.Duration, error)

	// SetWithTTL sets a value with specific TTL
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// FlushByPrefix removes all keys matching prefix
	FlushByPrefix(ctx context.Context, prefix string) error

	// Close closes the cache connection
	Close() error

	// Ping checks if cache is healthy
	Ping(ctx context.Context) error
}

// RedisCache implements Cache interface using Redis
type RedisCache struct {
	config *config.RedisConfig
	client *redis.Client
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(cfg *config.RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisCache{config: cfg, client: client}, nil
}

// Get retrieves a value by key
func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Set stores a value with optional TTL
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.SetWithTTL(ctx, key, value, ttl)
}

// SetWithTTL sets a value with specific TTL
func (r *RedisCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var strVal string
	switch v := value.(type) {
	case string:
		strVal = v
	case []byte:
		strVal = string(v)
	default:
		bytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		strVal = string(bytes)
	}

	return r.client.Set(ctx, key, strVal, ttl).Err()
}

// Delete removes a key
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Exists checks if a key exists
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetTTL returns remaining TTL for a key
func (r *RedisCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// FlushByPrefix removes all keys matching prefix
func (r *RedisCache) FlushByPrefix(ctx context.Context, prefix string) error {
	iter := r.client.Scan(ctx, 0, fmt.Sprintf("%s*", prefix), 0).Iterator()
	for iter.Next(ctx) {
		if err := r.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// Close closes the cache connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// Ping checks if cache is healthy
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Cache key prefixes
const (
	PrefixManga       = "manga:"
	PrefixUser        = "user:"
	PrefixSession     = "session:"
	PrefixRateLimit   = "ratelimit:"
	PrefixSearch      = "search:"
	PrefixExternal    = "external:"
	PrefixLeaderboard = "leaderboard:"
)

// BuildKey creates a cache key with prefix
func BuildKey(prefix, id string) string {
	return fmt.Sprintf("%s%s", prefix, id)
}

// Default TTLs
const (
	TTLShort  = 5 * time.Minute
	TTLMedium = 30 * time.Minute
	TTLLong   = 2 * time.Hour
	TTLDay    = 24 * time.Hour
)
