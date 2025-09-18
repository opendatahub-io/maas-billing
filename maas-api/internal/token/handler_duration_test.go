package token_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
	authv1 "k8s.io/api/authentication/v1"
)

func TestIssueToken_ExpirationFormats(t *testing.T) {
	tokenScenarios := map[string]fixtures.TokenReviewScenario{
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
		name                   string
		expiration             string
		expirationInRawSeconds bool
		expectedStatus         int
		expectedError          string
		shouldHaveToken        bool
		description            string
	}{
		// Valid durations
		{
			name:            "seconds format",
			expiration:      "601s", // Minimal expiration is 10 minutes.
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Standard seconds format should work",
		},
		{
			name:            "minutes format",
			expiration:      "15m",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Standard minutes format should work",
		},
		{
			name:            "hours format",
			expiration:      "2h",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Standard hours format should work",
		},
		{
			name:            "complex duration",
			expiration:      "1h30m45s",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Complex multi-unit duration should work",
		},
		{
			name:            "decimal duration",
			expiration:      "1.5h",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Decimal duration should work",
		},
		{
			name:            "default expiration (empty)",
			expiration:      "",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Empty expiration should default to 4h",
		},
		{
			name:            "zero expiration",
			expiration:      "0",
			expectedStatus:  http.StatusCreated,
			shouldHaveToken: true,
			description:     "Zero expiration should default to 4h",
		},
		{
			name:                   "zero expiration in raw seconds",
			expiration:             "0",
			expirationInRawSeconds: true,
			expectedStatus:         http.StatusCreated,
			shouldHaveToken:        true,
			description:            "Zero expiration should default to 4h",
		},
		// Invalid durations
		{
			name:                   "1 minute expiration",
			expiration:             "60",
			expirationInRawSeconds: true,
			expectedStatus:         http.StatusBadRequest,
			expectedError:          "token expiration must be at least 10 minutes",
			shouldHaveToken:        false,
			description:            "Minimal expiration is 10 minutes.",
		},
		{
			name:            "negative duration",
			expiration:      "-30h",
			expectedStatus:  http.StatusBadRequest,
			expectedError:   "must be a positive number",
			shouldHaveToken: false,
			description:     "Negative duration should be rejected",
		},
		{
			name:            "invalid unit",
			expiration:      "30x",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Invalid time unit should be rejected",
		},
		{
			name:            "no unit",
			expiration:      "30",
			expectedStatus:  http.StatusBadRequest,
			expectedError:   "invalid duration \"30\": must be a positive number ending in s, m, or h",
			shouldHaveToken: false,
			description:     "Number without unit should be rejected",
		},
		{
			name:            "invalid format",
			expiration:      "abc",
			expectedStatus:  http.StatusBadRequest,
			expectedError:   "invalid duration \"abc\"",
			shouldHaveToken: false,
			description:     "Non-numeric format should be rejected",
		},
		{
			name:            "spaces in duration",
			expiration:      "1 h",
			expectedStatus:  http.StatusBadRequest,
			shouldHaveToken: false,
			description:     "Spaces in duration should be rejected",
		},
		{
			name:            "decimal without unit",
			expiration:      "1.5",
			expectedStatus:  http.StatusBadRequest,
			expectedError:   "invalid duration \"1.5\": must be a positive number ending in s, m, or h",
			shouldHaveToken: false,
			description:     "Decimal without unit should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, reviewer, _ := fixtures.StubTokenProviderAPIs(t, true, tokenScenarios)
			router := fixtures.SetupTestRouter(manager, reviewer)

			w := httptest.NewRecorder()

			expiration := tt.expiration
			if !tt.expirationInRawSeconds {
				expiration = fmt.Sprintf("\"%s\"", expiration)
			}
			jsonPayload := fmt.Sprintf(`
{
			"expiration": %s
}`, expiration)

			request, _ := http.NewRequest("POST", "/v1/tokens", bytes.NewBuffer([]byte(jsonPayload)))
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
					t.Errorf("expected error for invalid Expiration. Description: %s", tt.description)
				}
				if !strings.Contains(response["error"].(string), tt.expectedError) {
					t.Errorf("expected error message: '%s'; got: '%v'\n", tt.expectedError, response["error"])
				}
			}
		})
	}
}
