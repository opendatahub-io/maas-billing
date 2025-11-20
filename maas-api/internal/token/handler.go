package token

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// ExtractUserInfo validates kubernetes tokens and checks against deny list
func (h *Handler) ExtractUserInfo(reviewer *Reviewer) gin.HandlerFunc {
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

		// Validate with Manager (K8s + Deny List)
		userContext, err := h.manager.ValidateToken(c.Request.Context(), token, reviewer)
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
func (h *Handler) IssueToken(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Expiration == nil {
		req.Expiration = &Duration{time.Hour * 4}
	}

	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)

	expiration := req.Expiration.Duration
	if expiration.Abs() != expiration {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expiration format, must be positive", "expiration": req.Expiration})
		return
	}

	token, err := h.manager.GenerateToken(c.Request.Context(), user, expiration, req.Name)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	response := Response{
		Token: token,
	}

	c.JSON(http.StatusCreated, response)
}

// ListTokens handles GET /v1/tokens
func (h *Handler) ListTokens(c *gin.Context) {
	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)
	tokens, err := h.manager.GetTokens(c.Request.Context(), user)
	if err != nil {
		log.Printf("Failed to list tokens for user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tokens"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// RevokeToken handles DELETE /v1/tokens/:id
func (h *Handler) RevokeToken(c *gin.Context) {
	tokenID := c.Param("id")
	if tokenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token ID required"})
		return
	}

	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)
	err := h.manager.RevokeToken(c.Request.Context(), user, tokenID)
	if err != nil {
		log.Printf("Failed to revoke token %s for user %s: %v", tokenID, user.Username, err)
		// 404 if not found?
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke token"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// RevokeAllTokens handles DELETE /v1/tokens
func (h *Handler) RevokeAllTokens(c *gin.Context) {
	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)
	err := h.manager.RevokeTokens(c.Request.Context(), user)
	if err != nil {
		log.Printf("Failed to revoke tokens for user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke tokens"})
		return
	}

	log.Printf("Successfully revoked all tokens for user %s", user.Username)
	c.JSON(http.StatusNoContent, nil)
}
