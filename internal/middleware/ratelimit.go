// Package middleware provides HTTP middleware for the OmniLedger API.
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiterConfig defines the request budget for a sliding window.
type RateLimiterConfig struct {
	MaxRequests int
	Window      time.Duration
}

// RateLimiter enforces per-IP request limits using a Redis sorted-set
// sliding window.
type RateLimiter struct {
	rdb    *redis.Client
	config RateLimiterConfig
}

// NewRateLimiter returns a RateLimiter backed by the given Redis client.
func NewRateLimiter(rdb *redis.Client, cfg RateLimiterConfig) *RateLimiter {
	return &RateLimiter{rdb: rdb, config: cfg}
}

// Limit wraps a handler with per-IP rate limiting scoped to the given route key.
// On failure to reach Redis, the request is allowed through (fail-open).
func (rl *RateLimiter) Limit(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ip := extractIP(r)
		key := fmt.Sprintf("vault:ratelimit:%s:%s", ip, route)
		now := time.Now()
		nowStr := strconv.FormatInt(now.UnixNano(), 10)
		windowStart := now.Add(-rl.config.Window)

		allowed, remaining, err := rl.check(ctx, key, nowStr, windowStart, now)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.MaxRequests))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(rl.config.Window).Unix(), 10))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(rl.config.Window.Seconds())))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("rate limit exceeded — max %d requests per %s", rl.config.MaxRequests, rl.config.Window),
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) check(ctx context.Context, key, nowStr string, windowStart, now time.Time) (bool, int, error) {
	pipe := rl.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(windowStart.UnixNano(), 10))
	cardCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: nowStr})
	pipe.Expire(ctx, key, rl.config.Window+time.Second)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, err
	}
	count := int(cardCmd.Val())
	if count >= rl.config.MaxRequests {
		rl.rdb.ZRem(ctx, key, nowStr)
		return false, 0, nil
	}
	remaining := rl.config.MaxRequests - count - 1
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining, nil
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
