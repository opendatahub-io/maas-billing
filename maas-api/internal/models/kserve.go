package models

import (
	"errors"
	"fmt"
	"log"

	kservev1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kservelistersv1alpha1 "github.com/kserve/kserve/pkg/client/listers/serving/v1alpha1"
	kservelistersv1beta1 "github.com/kserve/kserve/pkg/client/listers/serving/v1beta1"
	"github.com/openai/openai-go/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	gatewaylisters "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"
)

type Manager struct {
	isvcLister      kservelistersv1beta1.InferenceServiceLister
	llmIsvcLister   kservelistersv1alpha1.LLMInferenceServiceLister
	httpRouteLister gatewaylisters.HTTPRouteLister
	gatewayRef      GatewayRef
}

func NewManager(
	isvcLister kservelistersv1beta1.InferenceServiceLister,
	llmIsvcLister kservelistersv1alpha1.LLMInferenceServiceLister,
	httpRouteLister gatewaylisters.HTTPRouteLister,
	gatewayRef GatewayRef,
) (*Manager, error) {
	if isvcLister == nil {
		return nil, errors.New("isvcLister is required")
	}
	if llmIsvcLister == nil {
		return nil, errors.New("llmIsvcLister is required")
	}
	if httpRouteLister == nil {
		return nil, errors.New("httpRouteLister is required")
	}

	return &Manager{
		isvcLister:      isvcLister,
		llmIsvcLister:   llmIsvcLister,
		httpRouteLister: httpRouteLister,
		gatewayRef:      gatewayRef,
	}, nil
}

// ListAvailableModels lists all InferenceServices across all namespaces.
func (m *Manager) ListAvailableModels() ([]Model, error) {
	list, err := m.isvcLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list InferenceServices: %w", err)
	}

	return inferenceServicesToModels(list)
}

func inferenceServicesToModels(items []*kservev1beta1.InferenceService) ([]Model, error) {
	models := make([]Model, 0, len(items))

	for _, item := range items {
		url := findInferenceServiceURL(item)
		if url == nil {
			log.Printf("DEBUG: Failed to find URL for InferenceService %s/%s", item.Namespace, item.Name)
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
			Ready: checkInferenceServiceReadiness(item),
		})
	}

	return models, nil
}

func findInferenceServiceURL(is *kservev1beta1.InferenceService) *apis.URL {
	if is.Status.URL != nil {
		return is.Status.URL
	}

	if is.Status.Address != nil && is.Status.Address.URL != nil {
		return is.Status.Address.URL
	}

	log.Printf("DEBUG: No URL found for InferenceService %s/%s", is.Namespace, is.Name)
	return nil
}

func checkInferenceServiceReadiness(is *kservev1beta1.InferenceService) bool {
	if is.DeletionTimestamp != nil {
		return false
	}

	if is.Generation > 0 && is.Status.ObservedGeneration != is.Generation {
		log.Printf("DEBUG: observedGeneration %d is stale (expected %d), not ready yet",
			is.Status.ObservedGeneration, is.Generation)
		return false
	}

	if len(is.Status.Conditions) == 0 {
		log.Printf("DEBUG: No conditions found for InferenceService %s/%s", is.Namespace, is.Name)
		return false
	}

	for _, cond := range is.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}
