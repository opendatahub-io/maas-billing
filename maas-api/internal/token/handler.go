package token

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type TokenManager interface {
	GenerateToken(ctx context.Context, user *UserContext, expiration time.Duration, name string) (*Token, error)
	RevokeTokens(ctx context.Context, user *UserContext) error
	ValidateToken(ctx context.Context, token string, reviewer *Reviewer) (*UserContext, error)
	GetTokens(ctx context.Context, user *UserContext) ([]NamedToken, error)
	GetToken(ctx context.Context, user *UserContext, jti string) (*NamedToken, error)
}

type Handler struct {
	name    string
	manager TokenManager
}

func NewHandler(name string, manager TokenManager) *Handler {
	return &Handler{
		name:    name,
		manager: manager,
	}
}

// ExtractUserInfo validates kubernetes tokens.
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

		// Validate with Manager (K8s validation)
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

// IssueToken handles POST /v1/tokens for issuing ephemeral tokens
func (h *Handler) IssueToken(c *gin.Context) {
	var req Request
	// BindJSON will still parse the request body, but we'll ignore the name field.
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty request body for default expiration
		if !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if req.Expiration == nil {
		req.Expiration = &Duration{time.Hour * 4}
	}

	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user, ok := userCtx.(*UserContext)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context type"})
		return
	}

	expiration := req.Expiration.Duration
	if expiration <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expiration must be positive"})
		return
	}

	if expiration < 10*time.Minute {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token expiration must be at least 10 minutes", "provided_expiration": expiration.String()})
		return
	}

	// For ephemeral tokens, we explicitly pass an empty name.
	token, err := h.manager.GenerateToken(c.Request.Context(), user, expiration, "")
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

// CreateAPIKey handles POST /v1/api-keys for creating managed, persistent tokens
func (h *Handler) CreateAPIKey(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token name is required for api keys"})
		return
	}

	if req.Expiration == nil {
		req.Expiration = &Duration{time.Hour * 24 * 30} // Default to 30 days for API keys
	}

	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)

	expiration := req.Expiration.Duration
	if expiration <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expiration must be positive"})
		return
	}

	if expiration < 10*time.Minute {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token expiration must be at least 10 minutes", "provided_expiration": expiration.String()})
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

// ListAPIKeys handles GET /v1/api-keys
func (h *Handler) ListAPIKeys(c *gin.Context) {
	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user := userCtx.(*UserContext)
	tokens, err := h.manager.GetTokens(c.Request.Context(), user)
	if err != nil {
		log.Printf("Failed to list tokens for user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list api keys"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// GetAPIKey handles GET /v1/api-keys/:id
func (h *Handler) GetAPIKey(c *gin.Context) {
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
	token, err := h.manager.GetToken(c.Request.Context(), user, tokenID)
	if err != nil {
		// In a real implementation, manager should return typed errors or we check error message
		// For now, we'll assume "token not found" message implies 404
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
			return
		}
		log.Printf("Failed to get token %s for user %s: %v", tokenID, user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve API key"})
		return
	}

	c.JSON(http.StatusOK, token)
}

// RevokeAllTokens handles DELETE /v1/tokens.
func (h *Handler) RevokeAllTokens(c *gin.Context) {
	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user, ok := userCtx.(*UserContext)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context type"})
		return
	}
	err := h.manager.RevokeTokens(c.Request.Context(), user)
	if err != nil {
		log.Printf("Failed to revoke tokens for user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke tokens"})
		return
	}

	log.Printf("Successfully revoked all tokens for user %s", user.Username)
	c.JSON(http.StatusNoContent, nil)
}
