package api_keys

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type CreateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Expiration  *token.Duration `json:"expiration"`
}

type Response struct {
	Token       string `json:"token"`
	Expiration  string `json:"expiration"`
	ExpiresAt   int64  `json:"expiresAt"`
	JTI         string `json:"jti"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (h *Handler) CreateAPIKey(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token name is required for api keys"})
		return
	}

	if req.Expiration == nil {
		req.Expiration = &token.Duration{Duration: time.Hour * 24 * 30} // Default to 30 days
	}

	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user, ok := userCtx.(*token.UserContext)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context type"})
		return
	}

	expiration := req.Expiration.Duration
	if err := token.ValidateExpiration(expiration, 10*time.Minute); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tok, err := h.service.CreateAPIKey(c.Request.Context(), user, req.Name, req.Description, expiration)
	if err != nil {
		log.Printf("Failed to generate api key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate api key"})
		return
	}

	c.JSON(http.StatusCreated, Response{
		Token:       tok.Token.Token,
		Expiration:  tok.Expiration.String(),
		ExpiresAt:   tok.ExpiresAt,
		JTI:         tok.JTI,
		Name:        tok.Name,
		Description: tok.Description,
	})
}

func (h *Handler) ListAPIKeys(c *gin.Context) {
	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user, ok := userCtx.(*token.UserContext)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context type"})
		return
	}

	tokens, err := h.service.ListAPIKeys(c.Request.Context(), user)
	if err != nil {
		log.Printf("Failed to list api keys: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list api keys"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

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

	user, ok := userCtx.(*token.UserContext)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context type"})
		return
	}

	tok, err := h.service.GetAPIKey(c.Request.Context(), user, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
			return
		}
		log.Printf("Failed to get api key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve API key"})
		return
	}

	c.JSON(http.StatusOK, tok)
}

// RevokeAllTokens handles DELETE /v1/tokens.
func (h *Handler) RevokeAllTokens(c *gin.Context) {
	userCtx, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User context not found"})
		return
	}

	user, ok := userCtx.(*token.UserContext)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context type"})
		return
	}

	if err := h.service.RevokeAll(c.Request.Context(), user); err != nil {
		log.Printf("Failed to revoke all tokens for user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke tokens"})
		return
	}

	log.Printf("Successfully revoked all tokens for user %s", user.Username)
	c.Status(http.StatusNoContent)
}
