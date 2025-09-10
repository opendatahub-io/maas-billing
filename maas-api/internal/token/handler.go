package token

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultTTL       = "4h"
	validDurationMsg = "Valid time units are \"ns\", \"us\" (or \"µs\"), \"ms\", \"s\", \"m\", \"h\"."
)

type Handler struct {
	name    string
	manager *Manager
}

func NewHandler(name string, manager *Manager) *Handler {
	return &Handler{
		name:    name,
		manager: manager,
	}
}

// ExtractUserInfo validates kubernetes tokens
func ExtractUserInfo(reviewer *Reviewer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format. Use: Authorization: Bearer <token>"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		userContext, err := reviewer.ExtractUserInfo(c.Request.Context(), token)
		if err != nil {
			log.Printf("Token validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "validation failed"})
			c.Abort()
			return
		}

		if !userContext.IsAuthenticated {
			log.Printf("Token is not authenticated for user: %s", userContext.Username)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			c.Abort()
			return
		}

		c.Set("user", userContext)
		c.Next()
	}
}

// IssueToken handles POST /v1/tokens
func (g *Handler) IssueToken(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TTL == "" || req.TTL == "0" {
		req.TTL = defaultTTL
	}

	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)

	duration, err := time.ParseDuration(req.TTL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid TTL format: %v. %s", err, validDurationMsg), "ttl": req.TTL})
		return
	}

	if duration.Abs() != duration {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid TTL format, must be positive", "ttl": req.TTL})
		return
	}

	token := g.manager.GenerateSAToken(c.Request.Context(), user, duration)

	response := Response{
		Token: token,
	}

	c.JSON(http.StatusCreated, response)
}

// RevokeAllTokens handles DELETE /v1/tokens
func (g *Handler) RevokeAllTokens(c *gin.Context) {
	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)
	err := g.manager.RevokeUserTokens(c.Request.Context(), user)
	if err != nil {
		log.Printf("Failed to revoke tokens for user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke tokens"})
		return
	}

	log.Printf("Successfully revoked all tokens for user %s", user.Username)
	c.JSON(http.StatusNoContent, nil)
}
