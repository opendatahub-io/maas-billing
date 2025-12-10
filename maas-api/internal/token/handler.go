package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// parseGroupsHeader parses the X-MAAS-GROUP header which comes as a JSON array
// Format: "[\"group1\",\"group2\",\"group3\"]" (JSON-encoded array string)
func parseGroupsHeader(header string) ([]string, error) {
	if header == "" {
		return nil, errors.New("header is empty")
	}

	// Try to unmarshal as JSON array directly
	var groups []string
	if err := json.Unmarshal([]byte(header), &groups); err != nil {
		return nil, fmt.Errorf("failed to parse header as JSON array: %w", err)
	}

	if len(groups) == 0 {
		return nil, errors.New("no groups found in header")
	}

	// Trim whitespace from each group
	for i := range groups {
		groups[i] = strings.TrimSpace(groups[i])
	}

	return groups, nil
}

// ExtractUserInfo extracts user information from X-MAAS-* headers set by the auth policy.
func (h *Handler) ExtractUserInfo() gin.HandlerFunc {
	return func(c *gin.Context) {

		username := strings.TrimSpace(c.GetHeader("X-MAAS-USERNAME"))
		groupHeader := c.GetHeader("X-MAAS-GROUP")
		source := strings.TrimSpace(c.GetHeader("X-MAAS-SOURCE"))

		// Validate required headers exist and are not empty
		if username == "" {
			log.Printf("Missing or empty X-MAAS-USERNAME header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-MAAS-USERNAME header required and must not be empty"})
			c.Abort()
			return
		}

		if groupHeader == "" {
			log.Printf("Missing X-MAAS-GROUP header for user: %s", username)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-MAAS-GROUP header required"})
			c.Abort()
			return
		}

		if source == "" {
			log.Printf("Missing X-MAAS-SOURCE header for user: %s", username)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-MAAS-SOURCE header required"})
			c.Abort()
			return
		}

		// Parse groups from header - format: "[group1 group2 group3]"
		groups, err := parseGroupsHeader(groupHeader)
		if err != nil {
			log.Printf("ERROR: Failed to parse X-MAAS-GROUP header. Header value: %q, Error: %v", groupHeader, err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid X-MAAS-GROUP header format: %v. Check auth policy configuration.", err),
			})
			c.Abort()
			return
		}

		// Create UserContext from headers
		userContext := &UserContext{
			Username:        username,
			Groups:          groups,
			IsAuthenticated: true, // Headers are set by auth policy, so user is authenticated
			// UID and JTI are not available from headers, leave empty
		}

		log.Printf("DEBUG - Extracted user info from headers - Username: %s, Groups: %v, Source: %s",
			username, groups, source)

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
