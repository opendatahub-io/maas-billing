package token_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
)

// MockManager is a mock type for the Manager.
type MockManager struct {
	mock.Mock
}

func (m *MockManager) GenerateToken(ctx context.Context, user *token.UserContext, expiration time.Duration, name string) (*token.Token, error) {
	args := m.Called(ctx, user, expiration, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	tok, ok := args.Get(0).(*token.Token)
	if !ok {
		return nil, args.Error(1)
	}
	return tok, args.Error(1)
}

func (m *MockManager) ValidateToken(ctx context.Context, tokenStr string, reviewer *token.Reviewer) (*token.UserContext, error) {
	args := m.Called(ctx, tokenStr, reviewer)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	userCtx, ok := args.Get(0).(*token.UserContext)
	if !ok {
		return nil, args.Error(1)
	}
	return userCtx, args.Error(1)
}

func (m *MockManager) GetNamespaceForUser(ctx context.Context, user *token.UserContext) (string, error) {
	args := m.Called(ctx, user)
	return args.String(0), args.Error(1)
}

func TestAPIEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockManager := new(MockManager)
	handler := token.NewHandler("test", mockManager)

	// Middleware to inject user context
	injectUser := func(c *gin.Context) {
		c.Set("user", &token.UserContext{Username: "test-user"})
		c.Next()
	}

	router := gin.New()
	router.Use(injectUser)
	router.POST("/v1/tokens", handler.IssueToken)

	t.Run("IssueToken - Ephemeral", func(t *testing.T) {
		// Setup expectation
		expectedToken := &token.Token{Token: "ephemeral-jwt", ExpiresAt: time.Now().Add(1 * time.Hour).Unix()}
		mockManager.On("GenerateToken", mock.Anything, mock.Anything, mock.Anything, "").Return(expectedToken, nil).Once()

		reqBody, _ := json.Marshal(map[string]any{})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockManager.AssertExpectations(t)
	})
}
