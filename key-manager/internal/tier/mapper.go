package tier

import (
	"context"
	"fmt"
	"log"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	MappingConfigMap = "tier-to-group-mapping"
)

// Mapper handles tier-to-group mapping lookups
type Mapper struct {
	configMapClient corev1typed.ConfigMapInterface
}

func NewMapper(clientset kubernetes.Interface, namespace string) *Mapper {
	return &Mapper{
		configMapClient: clientset.CoreV1().ConfigMaps(namespace),
	}
}

// GetTierForGroup returns the tier that contains the specified group
func (m *Mapper) GetTierForGroup(ctx context.Context, group string) (string, error) {
	tiers, err := m.loadTierConfig(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("ConfigMap %s not found, defaulting group %s to 'free' tier",
				MappingConfigMap, group)
			return "free", nil
		}
		log.Printf("Failed to load tier configuration from ConfigMap %s: %v", MappingConfigMap, err)
		return "", fmt.Errorf("failed to load tier configuration: %w", err)
	}

	for _, tier := range tiers {
		if slices.Contains(tier.Groups, group) {
			return tier.Name, nil
		}
	}

	return "", &GroupNotFoundError{Group: group}
}

func (m *Mapper) loadTierConfig(ctx context.Context) ([]Tier, error) {
	cm, err := m.configMapClient.Get(ctx, MappingConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	configData, exists := cm.Data["tiers"]
	if !exists {
		log.Printf("tiers key not found in ConfigMap %s", MappingConfigMap)
		return nil, fmt.Errorf("tier to group mapping configuration not found")
	}

	var tiers []Tier
	if err := yaml.Unmarshal([]byte(configData), &tiers); err != nil {
		return nil, fmt.Errorf("failed to parse tier configuration: %w", err)
	}

	return tiers, nil
}
