package tier

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/constant"
)

// Mapper handles tier-to-group mapping lookups.
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

func (m *Mapper) Namespace(ctx context.Context, tier string) (string, error) {
	tiers, err := m.loadTierConfig(ctx)
	if err != nil {
		return "", err
	}

	for i := range tiers {
		if tiers[i].Name == tier {
			return m.ProjectedNsName(&tiers[i]), nil
		}
	}

	return "", fmt.Errorf("tier %s not found", tier)
}

// GetTierForGroups returns the highest level tier for a user with multiple group memberships.
//
// Returns error if no groups provided or no groups found in any tier.
// Returns "free" as default if mapping is missing (fallback).
func (m *Mapper) GetTierForGroups(ctx context.Context, groups ...string) (string, error) {
	if len(groups) == 0 {
		return "", errors.New("no groups provided")
	}

	tiers, err := m.loadTierConfig(ctx)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return "", fmt.Errorf("tier mapping not found, provide configuration in %s", constant.TierMappingConfigMap)
		}
		log.Printf("Failed to load tier configuration from ConfigMap %s: %v", constant.TierMappingConfigMap, err)
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

// ProjectedSAGroup returns the projected SA group for a tier.
func (m *Mapper) ProjectedSAGroup(tier *Tier) string {
	return fmt.Sprintf("system:serviceaccounts:%s", m.ProjectedNsName(tier))
}

func (m *Mapper) ProjectedNsName(tier *Tier) string {
	return fmt.Sprintf("%s-tier-%s", m.tenantName, tier.Name)
}

func (m *Mapper) loadTierConfig(ctx context.Context) ([]Tier, error) {
	cm, err := m.configMapClient.Get(ctx, constant.TierMappingConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	configData, exists := cm.Data["tiers"]
	if !exists {
		log.Printf("tiers key not found in ConfigMap %s", constant.TierMappingConfigMap)
		return nil, errors.New("tier to group mapping configuration not found")
	}

	var tiers []Tier
	if err := yaml.Unmarshal([]byte(configData), &tiers); err != nil {
		return nil, fmt.Errorf("failed to parse tier configuration: %w", err)
	}

	for i := range tiers {
		tier := &tiers[i]
		tier.Groups = append(tier.Groups, m.ProjectedSAGroup(tier))
	}

	return tiers, nil
}
