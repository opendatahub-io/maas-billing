package tier_test

import (
	"testing"

	"github.com/opendatahub-io/maas-billing/key-manager/internal/tier"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMapper_GetTierForGroup(t *testing.T) {
	ctx := t.Context()

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

	clientset := fake.NewSimpleClientset([]runtime.Object{configMap}...)

	mapper := tier.NewMapper(clientset, "test-namespace")

	tests := []struct {
		name          string
		group         string
		expectedTier  string
		expectedError bool
	}{
		{
			name:         "system authenticated group",
			group:        "system:authenticated",
			expectedTier: "free",
		},
		{
			name:         "premium user group",
			group:        "premium-users",
			expectedTier: "premium",
		},
		{
			name:         "beta tester group",
			group:        "beta-testers",
			expectedTier: "premium",
		},
		{
			name:         "enterprise user group",
			group:        "enterprise-users",
			expectedTier: "enterprise",
		},
		{
			name:         "admin user group",
			group:        "admin-users",
			expectedTier: "enterprise",
		},
		{
			name:          "unknown group returns error",
			group:         "unknown-group",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := mapper.GetTierForGroup(ctx, tt.group)

			if tt.expectedError && err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tier != tt.expectedTier {
				t.Errorf("expected tier %s, got %s", tt.expectedTier, tier)
			}
		})
	}
}

func TestMapper_GetTierForGroup_MissingConfigMap(t *testing.T) {
	ctx := t.Context()

	clientset := fake.NewSimpleClientset()

	mapper := tier.NewMapper(clientset, "test-namespace")

	// Should default to free tier when ConfigMap is missing (infrastructure issue)
	tier, err := mapper.GetTierForGroup(ctx, "any-group")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if tier != "free" {
		t.Errorf("expected default tier 'free', got %s", tier)
	}
}
