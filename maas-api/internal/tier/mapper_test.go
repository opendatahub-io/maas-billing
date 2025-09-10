package tier_test

import (
	"testing"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testNamespace = "test-namespace"
	testTenant    = "test-tenant"
)

func TestMapper_GetTierForGroups(t *testing.T) {
	ctx := t.Context()

	configMap := fixtures.CreateTierConfigMap(testNamespace)

	clientset := fake.NewSimpleClientset([]runtime.Object{configMap}...)
	mapper := tier.NewMapper(clientset, testTenant, testNamespace)

	tests := []struct {
		name          string
		groups        []string
		expectedTier  string
		expectedError bool
		description   string
	}{
		{
			name:         "single group - free tier",
			groups:       []string{"system:authenticated"},
			expectedTier: "free",
			description:  "User belongs to only free tier group",
		},
		{
			name:         "single group - premium tier",
			groups:       []string{"premium-users"},
			expectedTier: "premium",
			description:  "User belongs to only premium tier group",
		},
		{
			name:         "single group - enterprise tier",
			groups:       []string{"enterprise-users"},
			expectedTier: "enterprise",
			description:  "User belongs to only enterprise tier group",
		},
		{
			name:         "multiple groups - enterprise wins over free",
			groups:       []string{"system:authenticated", "enterprise-users"},
			expectedTier: "enterprise",
			description:  "User belongs to both free and enterprise - enterprise has higher level (20 > 1)",
		},
		{
			name:         "multiple groups - premium wins over free",
			groups:       []string{"free-users", "premium-users"},
			expectedTier: "premium",
			description:  "User belongs to both free and premium - premium has higher level (10 > 1)",
		},
		{
			name:         "multiple groups - enterprise wins over premium",
			groups:       []string{"premium-users", "enterprise-users"},
			expectedTier: "enterprise",
			description:  "User belongs to both premium and enterprise - enterprise has higher level (20 > 10)",
		},
		{
			name:         "multiple groups - enterprise wins over developer",
			groups:       []string{"developer-users", "enterprise-users"},
			expectedTier: "enterprise",
			description:  "User belongs to both developer and enterprise - enterprise has higher level (20 > 15)",
		},
		{
			name:         "multiple groups - developer wins over premium",
			groups:       []string{"premium-users", "developer-users"},
			expectedTier: "developer",
			description:  "User belongs to both premium and developer - developer has higher level (15 > 10)",
		},
		{
			name:         "three groups - enterprise wins",
			groups:       []string{"free-users", "premium-users", "enterprise-users"},
			expectedTier: "enterprise",
			description:  "User belongs to free, premium, and enterprise - enterprise has highest level (20)",
		},
		{
			name:         "all groups - enterprise wins",
			groups:       []string{"system:authenticated", "premium-users", "developer-users", "admin-users"},
			expectedTier: "enterprise",
			description:  "User belongs to groups across all tiers - enterprise has highest level (20)",
		},
		{
			name:          "no groups provided",
			groups:        []string{},
			expectedError: true,
			description:   "Empty groups array should return error",
		},
		{
			name:          "unknown groups",
			groups:        []string{"unknown-group-1", "unknown-group-2"},
			expectedError: true,
			description:   "Groups not found in any tier should return error",
		},
		{
			name:         "mix of known and unknown groups",
			groups:       []string{"premium-users", "unknown-group"},
			expectedTier: "premium",
			description:  "Should find tier for known group and ignore unknown ones",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappedTiers, err := mapper.GetTierForGroups(ctx, tt.groups...)

			if tt.expectedError && err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if mappedTiers != tt.expectedTier {
				t.Errorf("expected mappedTiers %s, got %s", tt.expectedTier, mappedTiers)
			}
		})
	}
}

func TestMapper_GetTierForGroups_MissingConfigMap(t *testing.T) {
	ctx := t.Context()

	clientset := fake.NewSimpleClientset()
	mapper := tier.NewMapper(clientset, testTenant, testNamespace)

	// Should default to free mappedTier when ConfigMap is missing
	mappedTier, err := mapper.GetTierForGroups(ctx, "any-group", "another-group")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if mappedTier != "free" {
		t.Errorf("expected default mappedTier 'free', got %s", mappedTier)
	}
}

func TestMapper_GetTierForGroups_SameLevels(t *testing.T) {
	ctx := t.Context()

	// Test case where two tiers have the same level
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tier.MappingConfigMap,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"tiers": `
- name: tier-a
  description: Tier A
  level: 10
  groups:
  - group-a
- name: tier-b
  description: Tier B
  level: 10
  groups:
  - group-b
`,
		},
	}

	clientset := fake.NewSimpleClientset([]runtime.Object{configMap}...)
	mapper := tier.NewMapper(clientset, testTenant, testNamespace)

	// When levels are equal, first tier found should win
	mappedTier, err := mapper.GetTierForGroups(ctx, "group-a", "group-b")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should return tier-a since it appears first in the config and has same level
	if mappedTier != "tier-a" {
		t.Errorf("expected mappedTier 'tier-a', got %s", mappedTier)
	}
}
