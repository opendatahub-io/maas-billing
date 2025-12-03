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
	ValidateToken(ctx context.Context, token string, reviewer *Reviewer) (*UserContext, error)
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

// IssueToken handles POST /v1/tokens for issuing ephemeral tokens.
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
	if err := ValidateExpiration(expiration, 10*time.Minute); err != nil {
		response := gin.H{"error": err.Error()}
		if expiration > 0 && expiration < 10*time.Minute {
			response["provided_expiration"] = expiration.String()
		}
		c.JSON(http.StatusBadRequest, response)
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
