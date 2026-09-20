package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// slidingWindowScript implements the sliding window counter algorithm
// atomically in Redis. It blends the previous window's count (weighted by
// how much of it still overlaps the sliding window) with the current
// window's count, giving a smooth approximation of a true sliding log
// without storing a timestamp per request.
//
// KEYS[1] = current window counter key
// KEYS[2] = previous window counter key
// ARGV[1] = limit (max requests per window)
// ARGV[2] = window size in milliseconds
// ARGV[3] = current timestamp in milliseconds
//
// Returns {allowed (0/1), remaining, retry_after_ms}
const slidingWindowScript = `
local current_key = KEYS[1]
local previous_key = KEYS[2]
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])

local current_window_start = now_ms - (now_ms % window_ms)
local elapsed_in_current = now_ms - current_window_start

local current_count = tonumber(redis.call("GET", current_key)) or 0
local previous_count = tonumber(redis.call("GET", previous_key)) or 0

local weight = (window_ms - elapsed_in_current) / window_ms
if weight < 0 then weight = 0 end

local estimated = (previous_count * weight) + current_count

if estimated >= limit then
    local ttl = redis.call("PTTL", current_key)
    if ttl < 0 then ttl = window_ms end
    return {0, 0, ttl}
end

local new_count = redis.call("INCR", current_key)
if new_count == 1 then
    redis.call("PEXPIRE", current_key, window_ms * 2)
end

local remaining = limit - math.floor(estimated) - 1
if remaining < 0 then remaining = 0 end

return {1, remaining, window_ms - elapsed_in_current}
`

// Result is the outcome of a single rate limit check.
type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// SlidingWindowLimiter evaluates the sliding-window-counter algorithm
// against Redis using a single atomic Lua script call per request, so
// concurrent requests never race on read-then-write.
type SlidingWindowLimiter struct {
	client *redis.Client
	script *redis.Script
}

func NewSlidingWindowLimiter(client *redis.Client) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		client: client,
		script: redis.NewScript(slidingWindowScript),
	}
}

// Allow checks whether a request identified by key is permitted under the
// given limit within windowSec seconds. Keys for the current and previous
// window use a Redis hash tag ({key}) so both always land on the same
// cluster slot, keeping the limiter safe to run against Redis Cluster.
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string, limit int, windowSec int) (*Result, error) {
	if limit <= 0 {
		return &Result{Allowed: true, Remaining: 0}, nil
	}
	if windowSec <= 0 {
		windowSec = 60
	}

	windowMs := int64(windowSec) * 1000
	now := time.Now().UnixMilli()
	currentWindow := now / windowMs
	previousWindow := currentWindow - 1

	currentKey := fmt.Sprintf("ratelimit:{%s}:%d", key, currentWindow)
	previousKey := fmt.Sprintf("ratelimit:{%s}:%d", key, previousWindow)

	res, err := l.script.Run(ctx, l.client, []string{currentKey, previousKey}, limit, windowMs, now).Result()
	if err != nil {
		return nil, fmt.Errorf("sliding window script failed: %w", err)
	}

	values, ok := res.([]interface{})
	if !ok || len(values) != 3 {
		return nil, fmt.Errorf("unexpected script response: %v", res)
	}

	allowed, _ := values[0].(int64)
	remaining, _ := values[1].(int64)
	retryMs, _ := values[2].(int64)

	return &Result{
		Allowed:    allowed == 1,
		Remaining:  int(remaining),
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}, nil
}
