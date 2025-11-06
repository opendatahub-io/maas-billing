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

// ListAvailableLLMs lists all LLMInferenceServices or InferenceServices.
// Tries standard InferenceServices first, then adds LLMInferenceServices if available.
func (m *Manager) ListAvailableLLMs(ctx context.Context) ([]Model, error) {
	// Try standard InferenceServices first (works on any Kubernetes with KServe)
	models, err := m.ListAvailableModels(ctx)
	if err != nil {
		return nil, err
	}

	// Try to add LLMInferenceServices if OpenShift AI is installed
	llmGVR := schema.GroupVersionResource{
		Group:    "serving.kserve.io",
		Version:  "v1alpha1",
		Resource: "llminferenceservices",
	}

	log.Printf("DEBUG: Attempting to list LLMInferenceServices")

	llmList, llmErr := m.k8sClient.Resource(llmGVR).
		Namespace(metav1.NamespaceAll).
		List(ctx, metav1.ListOptions{})

	if llmErr != nil {
		// Expected when OpenShift AI is not installed
		log.Printf("DEBUG: LLMInferenceServices not available: %v", llmErr)
		return models, nil
	}

	log.Printf("DEBUG: Found %d LLMInferenceServices", len(llmList.Items))

	// Add LLMInferenceServices to the list
	llmModels, err := toModels(llmList)
	if err != nil {
		log.Printf("WARN: Failed to parse LLMInferenceServices: %v", err)
		return models, nil // Return what we have
	}

	return append(models, llmModels...), nil
}

func toModels(list *unstructured.UnstructuredList) ([]Model, error) {
	models := make([]Model, 0, len(list.Items))

	for _, item := range list.Items {
		url, errURL := findURL(item)
		if errURL != nil {
			log.Printf("DEBUG: Failed to find URL for %s %s/%s: %v",
				item.GetKind(), item.GetNamespace(), item.GetName(), errURL)
		}

		// Default to metadata.name
		modelID := item.GetName()

		// Check if .spec.model.name exists
		if name, found, err := unstructured.NestedString(item.Object, "spec", "model", "name"); err != nil {
			log.Printf("DEBUG: Error reading spec.model.name for %s %s/%s: %v",
				item.GetKind(), item.GetNamespace(), item.GetName(), err)
		} else if found && name != "" {
			modelID = name
		}

		models = append(models, Model{
			Model: openai.Model{
				ID:      modelID,
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
	if s, ok, err := unstructured.NestedString(item.Object, "status", "url"); err != nil {
		return nil, fmt.Errorf("failed to read status.url for %s/%s: %w",
			item.GetNamespace(), item.GetName(), err)
	} else if ok && strings.TrimSpace(s) != "" {
		return apis.ParseURL(s)
	}

	// Fallback: scan status.addresses for the first usable URL.
	addresses, found, err := unstructured.NestedSlice(item.Object, "status", "addresses")
	if err != nil {
		return nil, fmt.Errorf("failed to read status.addresses for %s/%s: %w",
			item.GetNamespace(), item.GetName(), err)
	}
	if !found || len(addresses) == 0 {
		return nil, fmt.Errorf("no status.url and no status.addresses for %s/%s",
			item.GetNamespace(), item.GetName())
	}
	for _, a := range addresses {
		m, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if u, ok, _ := unstructured.NestedString(m, "url"); ok && strings.TrimSpace(u) != "" {
			return apis.ParseURL(u)
		}
	}

	return nil, fmt.Errorf("no usable URL in status.addresses for %s/%s",
		item.GetNamespace(), item.GetName())
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
