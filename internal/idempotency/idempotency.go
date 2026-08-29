package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Service handles idempotent request deduplication.
type Service struct {
	redis *redis.Client
	ttl   time.Duration
}

// NewService creates a new idempotency Service.
func NewService(redis *redis.Client) *Service {
	return &Service{
		redis: redis,
		ttl:   24 * time.Hour,
	}
}

// ResultCache holds a cached invocation result for deduplication.
type ResultCache struct {
	InvocationID string `json:"invocation_id"`
	Status       string `json:"status"`
	Result       string `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Check returns the cached result if this idempotency key was already processed.
// Returns ("", nil) if not found.
func (s *Service) Check(ctx context.Context, key string) (*ResultCache, error) {
	if key == "" {
		return nil, nil
	}

	val, err := s.redis.Get(ctx, "idem:"+key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checking idempotency key: %w", err)
	}

	var result ResultCache
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, fmt.Errorf("parsing cached result: %w", err)
	}

	return &result, nil
}

// Store saves the result of a processed invocation under its idempotency key.
func (s *Service) Store(ctx context.Context, key, invocationID, status, result, errMsg string) error {
	if key == "" {
		return nil
	}

	cache := ResultCache{
		InvocationID: invocationID,
		Status:       status,
		Result:       result,
		Error:        errMsg,
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	return s.redis.Set(ctx, "idem:"+key, data, s.ttl).Err()
}

// AcquireLock attempts to acquire a distributed lock for processing.
// This prevents race conditions when two requests with the same key arrive simultaneously.
func (s *Service) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, string) {
	if key == "" {
		return true, ""
	}

	lockKey := "lock:" + key
	token := uuid.New().String()

	// SET NX with TTL = atomic lock acquisition
	ok, err := s.redis.SetNX(ctx, lockKey, token, ttl).Result()
	if err != nil {
		return false, ""
	}

	return ok, token
}

// ReleaseLock releases the distributed lock.
func (s *Service) ReleaseLock(ctx context.Context, key, token string) error {
	if key == "" {
		return nil
	}

	// Use Lua script for atomic check-and-delete
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	_, err := script.Run(ctx, s.redis, []string{"lock:" + key}, token).Result()
	return err
}

// Delete removes an idempotency key (useful for cleanup).
func (s *Service) Delete(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	return s.redis.Del(ctx, "idem:"+key).Err()
}
