package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nimbusrun/nimbusrun/internal/auth"
)

// AuthMiddleware validates JWT and attaches user ID to context.
func AuthMiddleware(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		userID, tokenType, err := svc.ValidateToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token: " + err.Error()})
			return
		}

		if tokenType != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not an access token"})
			return
		}

		c.Set("user_id", userID.String())
		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT if present but doesn't require it.
func OptionalAuthMiddleware(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		userID, tokenType, err := svc.ValidateToken(parts[1])
		if err == nil && tokenType == "access" {
			c.Set("user_id", userID.String())
		}
		c.Next()
	}
}

// APIKeyMiddleware validates API key for programmatic access.
func APIKeyMiddleware(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(svc.APIKeyHeader())
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}
		// In full implementation, look up the user associated with this API key
		// For now, just verify format
		if !strings.HasPrefix(apiKey, "nimbus_") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}
		c.Next()
	}
}
