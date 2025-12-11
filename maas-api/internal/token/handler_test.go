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
	"github.com/stretchr/testify/require"

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

func (m *MockManager) GetNamespaceForUser(ctx context.Context, user *token.UserContext) (string, error) {
	args := m.Called(ctx, user)
	return args.String(0), args.Error(1)
}

func TestAPIEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockManager := new(MockManager)
	handler := token.NewHandler("test", mockManager)

	router := gin.New()
	router.Use(handler.ExtractUserInfo())
	router.POST("/v1/tokens", handler.IssueToken)

	t.Run("IssueToken - Ephemeral", func(t *testing.T) {
		// Setup expectation
		expectedToken := &token.Token{Token: "ephemeral-jwt", ExpiresAt: time.Now().Add(1 * time.Hour).Unix()}
		mockManager.On("GenerateToken", mock.Anything, mock.Anything, mock.Anything, "").Return(expectedToken, nil).Once()

		reqBody, _ := json.Marshal(map[string]any{})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))

		// Set required X-MAAS-* headers (using canonical format)
		req.Header.Set("X-Maas-Username", "test-user")
		req.Header.Set("X-Maas-Group", `["test-group","system:authenticated"]`)
		req.Header.Set("X-Maas-Source", "kubernetes-local")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockManager.AssertExpectations(t)
	})

	t.Run("IssueToken - Missing Username Header", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]any{})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))

		// Missing X-Maas-Username header
		req.Header.Set("X-Maas-Group", `["test-group"]`)
		req.Header.Set("X-Maas-Source", "kubernetes-local")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "X-MAAS-USERNAME header required")
	})

	t.Run("IssueToken - Missing Group Header", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]any{})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))

		req.Header.Set("X-Maas-Username", "test-user")
		// Missing X-Maas-Group header
		req.Header.Set("X-Maas-Source", "kubernetes-local")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "X-MAAS-GROUP header required")
	})

	t.Run("IssueToken - Missing Source Header", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]any{})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))

		req.Header.Set("X-Maas-Username", "test-user")
		req.Header.Set("X-Maas-Group", `["test-group"]`)
		// Missing X-Maas-Source header

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "X-MAAS-SOURCE header required")
	})

	t.Run("IssueToken - Invalid Group Header Format", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]any{})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))

		req.Header.Set("X-Maas-Username", "test-user")
		req.Header.Set("X-Maas-Group", "invalid-format") // Not a valid JSON array
		req.Header.Set("X-Maas-Source", "kubernetes-local")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "Invalid X-MAAS-GROUP header format")
	})
}
