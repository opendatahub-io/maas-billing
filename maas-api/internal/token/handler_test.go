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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/token"
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

	tests := []struct {
		name            string
		username        string
		group           string
		expectedStatus  int
		expectedError   string
		expectedCode    string
		shouldHaveToken bool
		description     string
	}{
		{
			name:            "Ephemeral",
			username:        "test-user",
			group:           `["test-group","system:authenticated"]`,
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Valid headers should issue token",
		},
		{
			name:           "Missing Username Header",
			username:       "",
			group:          `["test-group"]`,
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Exception thrown while generating token",
			expectedCode:   "ERR_7F3A",
			description:    "Missing username header should return 500",
		},
		{
			name:           "Missing Group Header",
			username:       "test-user",
			group:          "",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Exception thrown while generating token",
			expectedCode:   "ERR_B2C9",
			description:    "Missing group header should return 500",
		},
		{
			name:           "Invalid Group Header Format",
			username:       "test-user",
			group:          "invalid-format",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Exception thrown while generating token",
			expectedCode:   "ERR_C4D1",
			description:    "Invalid group header format should return 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldHaveToken {
				// Setup expectation for successful token generation
				expectedToken := &token.Token{Token: "ephemeral-jwt", ExpiresAt: time.Now().Add(1 * time.Hour).Unix()}
				mockManager.On("GenerateToken", mock.Anything, mock.Anything, mock.Anything, "").Return(expectedToken, nil).Once()
			}

			reqBody, _ := json.Marshal(map[string]any{})
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/tokens", bytes.NewBuffer(reqBody))

			// Set headers based on test case (using canonical format)
			if tt.username != "" {
				req.Header.Set("X-Maas-Username", tt.username)
			}
			if tt.group != "" {
				req.Header.Set("X-Maas-Group", tt.group)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Description: %s", tt.expectedStatus, w.Code, tt.description)
			}

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.shouldHaveToken {
				if response["token"] == nil || response["token"] == "" {
					t.Errorf("expected non-empty token. Description: %s", tt.description)
				}
				mockManager.AssertExpectations(t)
			} else {
				if response["error"] == nil {
					t.Errorf("expected error. Description: %s", tt.description)
				}
				errorMsg, ok := response["error"].(string)
				if !ok {
					t.Errorf("expected error to be a string, got %T", response["error"])
				} else if errorMsg != tt.expectedError {
					t.Errorf("expected error message: '%s'; got: '%s'. Description: %s", tt.expectedError, errorMsg, tt.description)
				}
				if tt.expectedCode != "" {
					exceptionCode, ok := response["exceptionCode"].(string)
					if !ok {
						t.Errorf("expected exceptionCode to be a string, got %T", response["exceptionCode"])
					} else if exceptionCode != tt.expectedCode {
						t.Errorf("expected exceptionCode: '%s'; got: '%s'. Description: %s", tt.expectedCode, exceptionCode, tt.description)
					}
				}
			}
		})
	}
}
