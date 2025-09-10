package tier_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/opendatahub-io/maas-billing/key-manager/internal/tier"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func setupTestRouter(mapper *tier.Mapper) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := tier.NewHandler(mapper)
	router.GET("/tiers/lookup", handler.GetTierLookup)

	return router
}

func createTestMapper(withConfigMap bool) *tier.Mapper {
	var objects []runtime.Object

	if withConfigMap {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tier.MappingConfigMap,
				Namespace: "test-namespace",
			},
			Data: map[string]string{
				"tiers": `
- name: free
  description: Free tier
  groups:
  - system:authenticated
- name: premium
  description: Premium tier
  groups:
  - premium-users
  - beta-testers
- name: enterprise
  description: Enterprise tier
  groups:
  - enterprise-users
  - admin-users
`,
			},
		}
		objects = append(objects, configMap)
	}

	clientset := fake.NewSimpleClientset(objects...)
	return tier.NewMapper(clientset, "test-namespace")
}

func TestHandler_GetTierLookup_Success(t *testing.T) {
	mapper := createTestMapper(true)
	router := setupTestRouter(mapper)

	tests := []struct {
		name         string
		group        string
		expectedTier string
		expectedCode int
	}{
		{
			name:         "system authenticated group",
			group:        "system:authenticated",
			expectedTier: "free",
			expectedCode: http.StatusOK,
		},
		{
			name:         "premium users group",
			group:        "premium-users",
			expectedTier: "premium",
			expectedCode: http.StatusOK,
		},
		{
			name:         "enterprise users group",
			group:        "enterprise-users",
			expectedTier: "enterprise",
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/tiers/lookup?group=%s", tt.group), nil)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, w.Code)
			}

			var response tier.LookupResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if response.Group != tt.group {
				t.Errorf("expected group %s, got %s", tt.group, response.Group)
			}

			if response.Tier != tt.expectedTier {
				t.Errorf("expected tier %s, got %s", tt.expectedTier, response.Tier)
			}

			// Response should contain group and tier
		})
	}
}

func TestHandler_GetTierLookup_GroupNotFound(t *testing.T) {
	mapper := createTestMapper(true)
	router := setupTestRouter(mapper)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tiers/lookup?group=unknown-group", nil)
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

	expectedMessage := "group unknown-group not found in any tier"
	if response.Message != expectedMessage {
		t.Errorf("expected message '%s', got '%s'", expectedMessage, response.Message)
	}
}

func TestHandler_GetTierLookup_MissingGroup(t *testing.T) {
	mapper := createTestMapper(true)
	router := setupTestRouter(mapper)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tiers/lookup", nil)
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
}

func TestHandler_GetTierLookup_ConfigMapMissing(t *testing.T) {
	mapper := createTestMapper(false) // No ConfigMap
	router := setupTestRouter(mapper)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tiers/lookup?group=any-group", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response tier.LookupResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Group != "any-group" {
		t.Errorf("expected group 'any-group', got %s", response.Group)
	}

	if response.Tier != "free" {
		t.Errorf("expected tier 'free', got %s", response.Tier)
	}

}
