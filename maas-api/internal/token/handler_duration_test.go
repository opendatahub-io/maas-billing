package token_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
	authv1 "k8s.io/api/authentication/v1"
)

const (
	testNamespace = "test-namespace"
	testTenant    = "test-tenant"
)

func TestIssueToken_TTLDurationScenarios(t *testing.T) {
	tokenScenarios := map[string]tokenReviewScenario{
		"duration-test-token": {
			Authenticated: true,
			UserInfo: authv1.UserInfo{
				Username: "duration-test@example.com",
				UID:      "duration-uid",
				Groups:   []string{"system:authenticated"},
			},
		},
	}

	tests := []struct {
		name            string
		ttl             string
		expectedStatus  int
		shouldHaveToken bool
		description     string
	}{
		// Valid durations
		{
			name:            "seconds format",
			ttl:             "30s",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Standard seconds format should work",
		},
		{
			name:            "minutes format",
			ttl:             "15m",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Standard minutes format should work",
		},
		{
			name:            "hours format",
			ttl:             "2h",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Standard hours format should work",
		},
		{
			name:            "complex duration",
			ttl:             "1h30m45s",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Complex multi-unit duration should work",
		},
		{
			name:            "default TTL (empty)",
			ttl:             "",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Empty TTL should default to 4h",
		},
		{
			name:            "zero TTL",
			ttl:             "0",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Zero TTL should default to 4h",
		},
		// Invalid durations
		{
			name:            "negative duration",
			ttl:             "-30h",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Negative duration should be rejected",
		},
		{
			name:            "invalid unit",
			ttl:             "30x",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Invalid time unit should be rejected",
		},
		{
			name:            "no unit",
			ttl:             "30",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Number without unit should be rejected",
		},
		{
			name:            "invalid format",
			ttl:             "abc",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Non-numeric format should be rejected",
		},
		{
			name:            "spaces in duration",
			ttl:             "1 h",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Spaces in duration should be rejected",
		},
		{
			name:            "decimal without unit",
			ttl:             "1.5",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Decimal without unit should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, reviewer, _ := createTestComponents(t, true, tokenScenarios)
			router := setupTestRouter(manager, reviewer)

			req := token.Request{TTL: tt.ttl}
			jsonBody, _ := json.Marshal(req)

			w := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/v1/tokens", bytes.NewBuffer(jsonBody))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer duration-test-token")
			router.ServeHTTP(w, request)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Description: %s", tt.expectedStatus, w.Code, tt.description)
			}

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if tt.shouldHaveToken {
				if response["token"] == nil || response["token"] == "" {
					t.Errorf("expected non-empty token. Description: %s", tt.description)
				}
			} else {
				if response["error"] == nil {
					t.Errorf("expected error for invalid TTL. Description: %s", tt.description)
				}
				if !strings.Contains(response["error"].(string), "Invalid TTL format") {
					t.Errorf("expected TTL format error, got '%v'. Description: %s", response["error"], tt.description)
				}
			}
		})
	}
}
