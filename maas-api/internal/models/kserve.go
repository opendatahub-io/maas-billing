package models

import (
	"context"
	"fmt"
	"log"

	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	kservev1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kserveclientv1alpha1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	kserveclientv1beta1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1beta1"
	"github.com/openai/openai-go/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// Manager handles model discovery and listing.
type Manager struct {
	v1beta1Client  kserveclientv1beta1.ServingV1beta1Interface
	v1alpha1Client kserveclientv1alpha1.ServingV1alpha1Interface
}

// NewManager creates a new model manager.
func NewManager(v1beta1Client kserveclientv1beta1.ServingV1beta1Interface, v1alpha1Client kserveclientv1alpha1.ServingV1alpha1Interface) *Manager {
	return &Manager{
		v1beta1Client:  v1beta1Client,
		v1alpha1Client: v1alpha1Client,
	}
}

// ListAvailableModels lists all InferenceServices across all namespaces.
func (m *Manager) ListAvailableModels(ctx context.Context) ([]Model, error) {
	list, err := m.v1beta1Client.InferenceServices(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list InferenceServices: %w", err)
	}

	return inferenceServicesToModels(list.Items)
}

// ListAvailableLLMs lists all LLMInferenceServices across all namespaces.
func (m *Manager) ListAvailableLLMs(ctx context.Context) ([]Model, error) {
	list, err := m.v1alpha1Client.LLMInferenceServices(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list LLMInferenceServices: %w", err)
	}

	return llmInferenceServicesToModels(list.Items)
}

func inferenceServicesToModels(items []kservev1beta1.InferenceService) ([]Model, error) {
	models := make([]Model, 0, len(items))

	for _, item := range items {
		url := findInferenceServiceURL(&item)
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
			Ready: checkInferenceServiceReadiness(&item),
		})
	}

	return models, nil
}

func llmInferenceServicesToModels(items []kservev1alpha1.LLMInferenceService) ([]Model, error) {
	models := make([]Model, 0, len(items))

	for _, item := range items {
		url := findLLMInferenceServiceURL(&item)
		if url == nil {
			log.Printf("DEBUG: Failed to find URL for LLMInferenceService %s/%s", item.Namespace, item.Name)
		}

		modelID := item.Name

		if item.Spec.Model.Name != nil && *item.Spec.Model.Name != "" {
			modelID = *item.Spec.Model.Name
		}

		models = append(models, Model{
			Model: openai.Model{
				ID:      modelID,
				Object:  "model",
				OwnedBy: item.Namespace,
				Created: item.CreationTimestamp.Unix(),
			},
			URL:   url,
			Ready: checkLLMInferenceServiceReadiness(&item),
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

func findLLMInferenceServiceURL(llm *kservev1alpha1.LLMInferenceService) *apis.URL {
	if llm.Status.URL != nil {
		return llm.Status.URL
	}

	if llm.Status.Address != nil && llm.Status.Address.URL != nil {
		return llm.Status.Address.URL
	}

	if len(llm.Status.Addresses) > 0 {
		return llm.Status.Addresses[0].URL
	}

	log.Printf("DEBUG: No URL found for LLMInferenceService %s/%s", llm.Namespace, llm.Name)
	return nil
}

func checkInferenceServiceReadiness(is *kservev1beta1.InferenceService) bool {
	if is.DeletionTimestamp != nil {
		return false
	}

	// If observedGeneration lags, status is stale
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

func checkLLMInferenceServiceReadiness(llm *kservev1alpha1.LLMInferenceService) bool {
	if llm.DeletionTimestamp != nil {
		return false
	}

	// If observedGeneration lags, status is stale
	if llm.Generation > 0 && llm.Status.ObservedGeneration != llm.Generation {
		log.Printf("DEBUG: observedGeneration %d is stale (expected %d), not ready yet",
			llm.Status.ObservedGeneration, llm.Generation)
		return false
	}

	if len(llm.Status.Conditions) == 0 {
		log.Printf("DEBUG: No conditions found for LLMInferenceService %s/%s", llm.Namespace, llm.Name)
		return false
	}

	for _, cond := range llm.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}
