package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/sanu1001/pingmate/config"
	"github.com/sanu1001/pingmate/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware returns a Gin handler function that validates JWT tokens.
// It takes AuthService so it can check the Redis blacklist via IsBlacklisted.
func AuthMiddleware(authSvc services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── Step 1: Extract token from header ──────────────────
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authorization header is required",
			})
			return
		}

		// Header must be: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authorization header format must be: Bearer <token>",
			})
			return
		}

		tokenString := parts[1]

		// ── Step 2: Verify signature and expiry ────────────────
		claims := &services.JWTClaims{}
		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(t *jwt.Token) (any, error) {
				// Reject tokens signed with unexpected algorithms
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(config.App.JWTSecret), nil
			},
		)

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		// ── Step 3: Check Redis blacklist ──────────────────────
		blacklisted, err := authSvc.IsBlacklisted(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "could not verify token status",
			})
			return
		}
		if blacklisted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "token has been invalidated, please login again",
			})
			return
		}

		// ── Step 4: Attach user info to context ────────────────
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)

		// ── Step 5: Pass control to the handler ────────────────
		c.Next()
	}
}
