package fixtures

import (
	"github.com/opendatahub-io/maas-billing/maas-api/internal/constant"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Standard tier configuration used across tests
const TierConfigYAML = `
- name: free
  description: Free tier
  level: 1
  groups:
  - system:authenticated
  - free-users
- name: premium
  description: Premium tier
  level: 10
  groups:
  - premium-users
  - beta-testers
- name: developer
  description: Developer tier
  level: 15
  groups:
  - developer-users
- name: enterprise
  description: Enterprise tier
  level: 20
  groups:
  - enterprise-users
  - admin-users
`

// CreateTierConfigMap creates a ConfigMap with tier configuration
func CreateTierConfigMap(namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.TierMappingConfigMap,
			Namespace: namespace,
		},
		Data: map[string]string{
			"tiers": TierConfigYAML,
		},
	}
}
