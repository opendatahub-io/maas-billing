package token

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
)

// MockManager is a mock type for the Manager
type MockManager struct {
	mock.Mock
}

func (m *MockManager) GenerateToken(ctx context.Context, user *UserContext, expiration time.Duration, name string) (*Token, error) {
	args := m.Called(ctx, user, expiration, name)
	return args.Get(0).(*Token), args.Error(1)
}

func (m *MockManager) GetTokens(ctx context.Context, user *UserContext) ([]NamedToken, error) {
	args := m.Called(ctx, user)
	return args.Get(0).([]NamedToken), args.Error(1)
}

func (m *MockManager) GetToken(ctx context.Context, user *UserContext, jti string) (*NamedToken, error) {
	args := m.Called(ctx, user, jti)
	return args.Get(0).(*NamedToken), args.Error(1)
}

func (m *MockManager) RevokeTokens(ctx context.Context, user *UserContext) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockManager) ValidateToken(ctx context.Context, token string, reviewer *Reviewer) (*UserContext, error) {
	args := m.Called(ctx, token, reviewer)
	return args.Get(0).(*UserContext), args.Error(1)
}

func TestAPIEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockManager := new(MockManager)
	handler := NewHandler("test", mockManager)

	// Middleware to inject user context
	injectUser := func(c *gin.Context) {
		c.Set("user", &UserContext{Username: "test-user"})
		c.Next()
	}

	router := gin.New()
	router.Use(injectUser)
	router.POST("/v1/tokens", handler.IssueToken)
	router.POST("/v1/api-keys", handler.CreateAPIKey)
	router.GET("/v1/api-keys", handler.ListAPIKeys)
	router.GET("/v1/api-keys/:id", handler.GetAPIKey)
	router.DELETE("/v1/tokens", handler.RevokeAllTokens)

	t.Run("IssueToken - Ephemeral", func(t *testing.T) {
		// Setup expectation
		expectedToken := &Token{Token: "ephemeral-jwt", ExpiresAt: time.Now().Add(1 * time.Hour).Unix()}
		mockManager.On("GenerateToken", mock.Anything, mock.Anything, mock.Anything, "").Return(expectedToken, nil).Once()

		reqBody, _ := json.Marshal(Request{})
		req, _ := http.NewRequest(http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockManager.AssertExpectations(t)
	})

	t.Run("CreateAPIKey - Managed", func(t *testing.T) {
		// Setup expectation
		expectedToken := &Token{Token: "managed-jwt", ExpiresAt: time.Now().Add(24 * time.Hour).Unix()}
		mockManager.On("GenerateToken", mock.Anything, mock.Anything, mock.Anything, "my-key").Return(expectedToken, nil).Once()

		reqBody, _ := json.Marshal(Request{Name: "my-key"})
		req, _ := http.NewRequest(http.MethodPost, "/v1/api-keys", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockManager.AssertExpectations(t)
	})

	t.Run("ListAPIKeys", func(t *testing.T) {
		// Setup expectation
		expectedKeys := []NamedToken{{ID: "jti123", Name: "my-key"}}
		mockManager.On("GetTokens", mock.Anything, mock.Anything).Return(expectedKeys, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/v1/api-keys", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockManager.AssertExpectations(t)
	})

	t.Run("GetAPIKey", func(t *testing.T) {
		// Setup expectation
		expectedKey := &NamedToken{ID: "jti123", Name: "my-key"}
		mockManager.On("GetToken", mock.Anything, mock.Anything, "jti123").Return(expectedKey, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/v1/api-keys/jti123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockManager.AssertExpectations(t)
	})

	t.Run("RevokeAllTokens", func(t *testing.T) {
		// Setup expectation
		mockManager.On("RevokeTokens", mock.Anything, mock.Anything).Return(nil).Once()

		req, _ := http.NewRequest(http.MethodDelete, "/v1/tokens", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockManager.AssertExpectations(t)
	})
}
