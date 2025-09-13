package tier_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func setupTestRouter(mapper *tier.Mapper) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := tier.NewHandler(mapper)
	router.POST("/tiers/lookup", handler.TierLookup)

	return router
}

func createTestMapper(withConfigMap bool) *tier.Mapper {
	var objects []runtime.Object

	if withConfigMap {
		configMap := fixtures.CreateTierConfigMap(testNamespace)
		objects = append(objects, configMap)
	}

	clientset := fake.NewSimpleClientset(objects...)
	return tier.NewMapper(clientset, testTenant, testNamespace)
}

func TestHandler_PostTierLookup_Success(t *testing.T) {
	mapper := createTestMapper(true)
	router := setupTestRouter(mapper)

	tests := []struct {
		name         string
		groups       []string
		expectedTier string
		expectedCode int
	}{
		{
			name:         "single group - free tier",
			groups:       []string{"system:authenticated"},
			expectedTier: "free",
			expectedCode: http.StatusOK,
		},
		{
			name:         "single group - premium tier",
			groups:       []string{"premium-users"},
			expectedTier: "premium",
			expectedCode: http.StatusOK,
		},
		{
			name:         "single group - enterprise tier",
			groups:       []string{"enterprise-users"},
			expectedTier: "enterprise",
			expectedCode: http.StatusOK,
		},
		{
			name:         "multiple groups - enterprise wins over free",
			groups:       []string{"system:authenticated", "enterprise-users"},
			expectedTier: "enterprise",
			expectedCode: http.StatusOK,
		},
		{
			name:         "multiple groups - premium wins over free",
			groups:       []string{"system:authenticated", "premium-users"},
			expectedTier: "premium",
			expectedCode: http.StatusOK,
		},
		{
			name:         "all tiers - enterprise wins",
			groups:       []string{"system:authenticated", "premium-users", "admin-users"},
			expectedTier: "enterprise",
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := tier.LookupRequest{Groups: tt.groups}
			jsonBody, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/tiers/lookup", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, w.Code)
			}

			var response tier.LookupResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if response.Tier != tt.expectedTier {
				t.Errorf("expected tier %s, got %s", tt.expectedTier, response.Tier)
			}
		})
	}
}

func TestHandler_PostTierLookup_GroupNotFound(t *testing.T) {
	mapper := createTestMapper(true)
	router := setupTestRouter(mapper)

	reqBody := tier.LookupRequest{Groups: []string{"unknown-group"}}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tiers/lookup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response tier.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if response.Error != "not_found" {
		t.Errorf("expected error 'not_found', got '%s'", response.Error)
	}
}

func TestHandler_PostTierLookup_BadRequest(t *testing.T) {
	mapper := createTestMapper(true)
	router := setupTestRouter(mapper)

	tests := []struct {
		name        string
		requestBody string
		description string
	}{
		{
			name:        "empty request body",
			requestBody: "",
			description: "No JSON body provided",
		},
		{
			name:        "invalid JSON",
			requestBody: "{invalid json}",
			description: "Malformed JSON body",
		},
		{
			name:        "missing groups field",
			requestBody: "{}",
			description: "Request without groups field",
		},
		{
			name:        "empty groups array",
			requestBody: `{"groups": []}`,
			description: "Empty groups array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/tiers/lookup", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}

			var response tier.ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal error response: %v", err)
			}

			if response.Error != "bad_request" {
				t.Errorf("expected error 'bad_request', got '%s'", response.Error)
			}
		})
	}
}

func TestHandler_PostTierLookup_ConfigMapMissing_ShouldDefaultEveryUserToFreeTier(t *testing.T) {
	mapper := createTestMapper(false) // No ConfigMap
	router := setupTestRouter(mapper)

	reqBody := tier.LookupRequest{Groups: []string{"any-group"}}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tiers/lookup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response tier.LookupResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Tier != "free" {
		t.Errorf("expected tier 'free', got %s", response.Tier)
	}
}
