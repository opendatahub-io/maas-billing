package tier

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	MappingConfigMap = "tier-to-group-mapping"
)

var defaultTier = Tier{
	Name:  "free",
	Level: 0,
	Groups: []string{
		"system:authenticated",
	},
}

// Mapper handles tier-to-group mapping lookups
type Mapper struct {
	tenantName      string
	configMapClient corev1typed.ConfigMapInterface
}

func NewMapper(clientset kubernetes.Interface, tenantName, namespace string) *Mapper {
	return &Mapper{
		tenantName:      tenantName,
		configMapClient: clientset.CoreV1().ConfigMaps(namespace),
	}
}

func (m *Mapper) Namespaces(ctx context.Context) map[string]string {
	tiers, err := m.loadTierConfig(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			tiers = []Tier{defaultTier}
		}
	}

	namespaces := make(map[string]string, len(tiers))

	for _, tier := range tiers {
		namespaces[tier.Name] = fmt.Sprintf("%s-tier-%s", m.tenantName, tier.Name)
	}

	return namespaces
}

// GetTierForGroups returns the highest level tier for a user with multiple group memberships.
//
// Returns error if no groups provided or no groups found in any tier.
// Returns "free" as default if ConfigMap is missing (fallback).
func (m *Mapper) GetTierForGroups(ctx context.Context, groups ...string) (string, error) {
	if len(groups) == 0 {
		return "", fmt.Errorf("no groups provided")
	}

	tiers, err := m.loadTierConfig(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("ConfigMap %s not found, defaulting to 'free' tier", MappingConfigMap)
			return "free", nil
		}
		log.Printf("Failed to load tier configuration from ConfigMap %s: %v", MappingConfigMap, err)
		return "", fmt.Errorf("failed to load tier configuration: %w", err)
	}

	sort.SliceStable(tiers, func(i, j int) bool {
		return tiers[i].Level > tiers[j].Level
	})

	for _, tier := range tiers {
		for _, userGroup := range groups {
			if slices.Contains(tier.Groups, userGroup) {
				return tier.Name, nil
			}
		}
	}

	return "", &GroupNotFoundError{Group: fmt.Sprintf("groups [%s]", strings.Join(groups, ", "))}
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
