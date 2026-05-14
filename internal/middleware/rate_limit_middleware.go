package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sanu1001/pingmate/config"

	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware limits requests per user per time window.
// limit  = max requests allowed in the window
// window = duration of the window (e.g. 1 minute)
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// user_id is set by AuthMiddleware — always present on protected routes
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

		ctx := context.Background()
		key := fmt.Sprintf("rate:%s", userID)

		// Increment the counter for this user
		count, err := config.Redis.Incr(ctx, key).Result()
		if err != nil {
			// Redis error — fail open (allow request) rather than
			// blocking all users because of an infra hiccup
			c.Next()
			return
		}

		// First request in this window — set the expiry
		if count == 1 {
			config.Redis.Expire(ctx, key, window)
		}

		// Get remaining TTL to send helpful headers
		ttl, _ := config.Redis.TTL(ctx, key).Result()

		// Set rate limit headers so clients know their status
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(limit)-count)))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", int(ttl.Seconds())))

		// Over the limit — reject
		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": fmt.Sprintf("%.0f seconds", ttl.Seconds()),
			})
			return
		}

		c.Next()
	}
}

// max returns the larger of two int64 values.
// Defined here to avoid Go version compatibility issues.
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
