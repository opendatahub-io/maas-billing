package models

import (
	"context"
	"fmt"

	kservev1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kserveclientv1alpha1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	kserveclientv1beta1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1beta1"
	"github.com/openai/openai-go/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/logger"
)

// Manager handles model discovery and listing.
type Manager struct {
	v1beta1Client  kserveclientv1beta1.ServingV1beta1Interface
	v1alpha1Client kserveclientv1alpha1.ServingV1alpha1Interface
	gatewayClient  gatewayclient.GatewayV1Interface
	gatewayRef     GatewayRef
	logger         *logger.Logger
}

// NewManager creates a new model manager.
func NewManager(
	v1beta1Client kserveclientv1beta1.ServingV1beta1Interface,
	v1alpha1Client kserveclientv1alpha1.ServingV1alpha1Interface,
	gatewayClient gatewayclient.GatewayV1Interface,
	gatewayRef GatewayRef,
	log *logger.Logger,
) *Manager {
	return &Manager{
		v1beta1Client:  v1beta1Client,
		v1alpha1Client: v1alpha1Client,
		gatewayClient:  gatewayClient,
		gatewayRef:     gatewayRef,
		logger:         log,
	}
}

// ListAvailableModels lists all InferenceServices across all namespaces.
func (m *Manager) ListAvailableModels(ctx context.Context) ([]Model, error) {
	list, err := m.v1beta1Client.InferenceServices(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list InferenceServices: %w", err)
	}

	return m.inferenceServicesToModels(ctx, list.Items)
}

func (m *Manager) inferenceServicesToModels(ctx context.Context, items []kservev1beta1.InferenceService) ([]Model, error) {
	models := make([]Model, 0, len(items))
	log := m.logger.WithContext(ctx)

	for _, item := range items {
		url := m.findInferenceServiceURL(ctx, &item)
		if url == nil {
			log.Debug("Failed to find URL for InferenceService",
				"namespace", item.Namespace,
				"name", item.Name,
			)
		}

		modelID := item.Name
		if item.Spec.Predictor.Model != nil && item.Spec.Predictor.Model.ModelFormat.Name != "" {
			modelID = item.Spec.Predictor.Model.ModelFormat.Name
		}

		models = append(models, Model{
			Model: openai.Model{
				ID:      modelID,
				Object:  "model",
				OwnedBy: item.Namespace,
				Created: item.CreationTimestamp.Unix(),
			},
			URL:   url,
			Ready: m.checkInferenceServiceReadiness(ctx, &item),
		})
	}

	return models, nil
}

func (m *Manager) findInferenceServiceURL(ctx context.Context, is *kservev1beta1.InferenceService) *apis.URL {
	if is.Status.URL != nil {
		return is.Status.URL
	}

	if is.Status.Address != nil && is.Status.Address.URL != nil {
		return is.Status.Address.URL
	}

	m.logger.WithContext(ctx).Debug("No URL found for InferenceService",
		"namespace", is.Namespace,
		"name", is.Name,
	)
	return nil
}

func (m *Manager) checkInferenceServiceReadiness(ctx context.Context, is *kservev1beta1.InferenceService) bool {
	if is.DeletionTimestamp != nil {
		return false
	}

	if is.Generation > 0 && is.Status.ObservedGeneration != is.Generation {
		m.logger.WithContext(ctx).Debug("ObservedGeneration is stale, not ready yet",
			"namespace", is.Namespace,
			"name", is.Name,
			"observed_generation", is.Status.ObservedGeneration,
			"expected_generation", is.Generation,
		)
		return false
	}

	if len(is.Status.Conditions) == 0 {
		m.logger.WithContext(ctx).Debug("No conditions found for InferenceService",
			"namespace", is.Namespace,
			"name", is.Name,
		)
		return false
	}

	for _, cond := range is.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}
