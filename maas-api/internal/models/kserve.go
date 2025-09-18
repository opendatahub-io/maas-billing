package models

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/openai/openai-go/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/apis"
)

// Manager handles model discovery and listing
type Manager struct {
	k8sClient dynamic.Interface
}

// NewManager creates a new model manager
func NewManager(k8sClient dynamic.Interface) *Manager {
	return &Manager{
		k8sClient: k8sClient,
	}
}

// ListAvailableModels lists all InferenceServices across all namespaces
func (m *Manager) ListAvailableModels(ctx context.Context) ([]Model, error) {

	inferenceServiceGVR := schema.GroupVersionResource{
		Group:    "serving.kserve.io",
		Version:  "v1beta1",
		Resource: "inferenceservices",
	}

	log.Printf("DEBUG: Attempting to list InferenceServices with GVR: %+v", inferenceServiceGVR)

	list, err := m.k8sClient.Resource(inferenceServiceGVR).
		Namespace(metav1.NamespaceAll).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("DEBUG: Failed to list InferenceServices: %v", err)
		return nil, fmt.Errorf("failed to list InferenceServices: %w", err)
	}

	log.Printf("DEBUG: Found %d InferenceServices", len(list.Items))

	return toModels(list)
}

// ListAvailableLLMs lists all LLMInferenceServices across all namespaces.
func (m *Manager) ListAvailableLLMs(ctx context.Context) ([]Model, error) {
	llmGVR := schema.GroupVersionResource{
		Group:    "serving.kserve.io",
		Version:  "v1alpha1",
		Resource: "llminferenceservices",
	}

	log.Printf("DEBUG: Attempting to list LLMInferenceServices with GVR: %+v", llmGVR)

	list, err := m.k8sClient.Resource(llmGVR).
		Namespace(metav1.NamespaceAll).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("DEBUG: Failed to list LLMInferenceServices: %v", err)
		return nil, fmt.Errorf("failed to list LLMInferenceServices: %w", err)
	}

	log.Printf("DEBUG: Found %d LLMInferenceServices", len(list.Items))

	return toModels(list)
}

func toModels(list *unstructured.UnstructuredList) ([]Model, error) {
	models := make([]Model, 0, len(list.Items))

	for _, item := range list.Items {
		url, errURL := findURL(item)
		if errURL != nil {
			log.Printf("DEBUG: Failed to find URL for %s: %v", item.GetKind(), errURL)
		}

		models = append(models, Model{
			Model: openai.Model{
				ID:      item.GetName(),
				Object:  "model",
				OwnedBy: item.GetNamespace(),
				Created: item.GetCreationTimestamp().Unix(),
			},
			URL:   url,
			Ready: checkReadiness(item),
		})
	}

	return models, nil
}

func findURL(item unstructured.Unstructured) (*apis.URL, error) {
	status, found := item.Object["status"].(map[string]any)
	if !found {
		return nil, fmt.Errorf("failed to find status of %s/%s", item.GetNamespace(), item.GetName())
	}

	if url, ok := status["url"].(string); ok {
		return apis.ParseURL(url)
	}

	return nil, fmt.Errorf("failed to find URL in status of %s/%s", item.GetNamespace(), item.GetName())
}

func checkReadiness(item unstructured.Unstructured) bool {
	if item.GetDeletionTimestamp() != nil {
		return false
	}

	// If observedGeneration lags, status is stale, might not be ready yet.
	if gen := item.GetGeneration(); gen > 0 {
		if og, found, _ := unstructured.NestedInt64(item.Object, "status", "observedGeneration"); found && og < gen {
			log.Printf("DEBUG: observedGeneration %d is stale, not ready yet", og)
			return false
		}
	}

	conds, found, err := unstructured.NestedSlice(item.Object, "status", "conditions")
	if err != nil {
		log.Printf("ERROR: Failed to find conditions: %v", err)
		return false
	}
	if !found || len(conds) == 0 {
		log.Printf("DEBUG: No conditions found")
		return false
	}

	// Ensure all conditions have the status "True"
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}

		// Default is not ready if status field missing
		status := "false"
		if s, ok, _ := unstructured.NestedString(m, "status"); ok {
			status = strings.ToLower(s)
		} else if b, ok, _ := unstructured.NestedBool(m, "status"); ok {
			if b {
				status = "true"
			}
		}

		if status != "true" {
			return false
		}
	}

	return true
}
