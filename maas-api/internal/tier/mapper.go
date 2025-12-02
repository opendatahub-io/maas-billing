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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/constant"
)

// Mapper handles tier-to-group mapping lookups.
type Mapper struct {
	tenantName      string
	namespace       string
	configMapLister corelisters.ConfigMapLister
}

func NewMapper(ctx context.Context, clientset kubernetes.Interface, tenantName, namespace string) *Mapper {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		constant.DefaultResyncPeriod,
		informers.WithNamespace(namespace),
	)

	configMapInformer := informerFactory.Core().V1().ConfigMaps()
	configMapLister := configMapInformer.Lister()

	informerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), configMapInformer.Informer().HasSynced) {
		log.Fatalf("failed to wait for caches to sync")
	}

	return &Mapper{
		tenantName:      tenantName,
		namespace:       namespace,
		configMapLister: configMapLister,
	}
}

func (m *Mapper) Namespace(tier string) (string, error) {
	tiers, err := m.loadTierConfig()
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
func (m *Mapper) GetTierForGroups(groups ...string) (string, error) {
	if len(groups) == 0 {
		return "", errors.New("no groups provided")
	}

	tiers, err := m.loadTierConfig()
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

func (m *Mapper) loadTierConfig() ([]Tier, error) {
	cm, err := m.configMapLister.ConfigMaps(m.namespace).Get(constant.TierMappingConfigMap)
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
