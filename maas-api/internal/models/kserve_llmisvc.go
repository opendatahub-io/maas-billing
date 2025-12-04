package models

import (
	"context"
	"fmt"
	"log"

	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/openai/openai-go/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
)

type GatewayRef struct {
	Name      string
	Namespace string
}

func (m *Manager) ListAvailableLLMs(ctx context.Context) ([]Model, error) {
	list, err := m.v1alpha1Client.LLMInferenceServices(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list LLMInferenceServices: %w", err)
	}

	var instanceLLMs []kservev1alpha1.LLMInferenceService
	for i := range list.Items {
		llmIsvc := &list.Items[i]
		if m.partOfMaaSInstance(ctx, llmIsvc) {
			instanceLLMs = append(instanceLLMs, *llmIsvc)
		}
	}

	return llmInferenceServicesToModels(instanceLLMs)
}

// partOfMaaSInstance checks if the given LLMInferenceService is part of this "MaaS instance". This means that it is
// either directly referenced by the gateway that has MaaS capabilities, or it is referenced by an HTTPRoute that is managed by the gateway.
// The gateway is part of the component configuration.
func (m *Manager) partOfMaaSInstance(ctx context.Context, llmIsvc *kservev1alpha1.LLMInferenceService) bool {
	if llmIsvc.Spec.Router == nil {
		return false
	}

	if m.hasDirectGatewayReference(llmIsvc) {
		return true
	}

	if m.hasHTTPRouteSpecRefToGateway(llmIsvc) {
		return true
	}

	if m.hasReferencedRouteAttachedToGateway(ctx, llmIsvc) {
		return true
	}

	if m.hasManagedRouteAttachedToGateway(ctx, llmIsvc) {
		return true
	}

	return false
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

func findLLMInferenceServiceURL(llmIsvc *kservev1alpha1.LLMInferenceService) *apis.URL {
	if llmIsvc.Status.URL != nil {
		return llmIsvc.Status.URL
	}

	if llmIsvc.Status.Address != nil && llmIsvc.Status.Address.URL != nil {
		return llmIsvc.Status.Address.URL
	}

	if len(llmIsvc.Status.Addresses) > 0 {
		return llmIsvc.Status.Addresses[0].URL
	}

	log.Printf("DEBUG: No URL found for LLMInferenceService %s/%s", llmIsvc.Namespace, llmIsvc.Name)
	return nil
}

func checkLLMInferenceServiceReadiness(llmIsvc *kservev1alpha1.LLMInferenceService) bool {
	if llmIsvc.DeletionTimestamp != nil {
		return false
	}

	if llmIsvc.Generation > 0 && llmIsvc.Status.ObservedGeneration != llmIsvc.Generation {
		log.Printf("DEBUG: observedGeneration %d is stale (expected %d), not ready yet",
			llmIsvc.Status.ObservedGeneration, llmIsvc.Generation)
		return false
	}

	if len(llmIsvc.Status.Conditions) == 0 {
		log.Printf("DEBUG: No conditions found for LLMInferenceService %s/%s", llmIsvc.Namespace, llmIsvc.Name)
		return false
	}

	for _, cond := range llmIsvc.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}

func (m *Manager) hasDirectGatewayReference(llmIsvc *kservev1alpha1.LLMInferenceService) bool {
	if llmIsvc.Spec.Router.Gateway == nil {
		return false
	}

	for _, ref := range llmIsvc.Spec.Router.Gateway.Refs {
		if string(ref.Name) != m.gatewayRef.Name {
			continue
		}

		refNamespace := llmIsvc.Namespace
		if ref.Namespace != "" {
			refNamespace = string(ref.Namespace)
		}

		if refNamespace == m.gatewayRef.Namespace {
			return true
		}
	}

	return false
}

func (m *Manager) hasHTTPRouteSpecRefToGateway(llmIsvc *kservev1alpha1.LLMInferenceService) bool {
	if llmIsvc.Spec.Router.Route == nil || llmIsvc.Spec.Router.Route.HTTP == nil || llmIsvc.Spec.Router.Route.HTTP.Spec == nil {
		return false
	}

	for _, parentRef := range llmIsvc.Spec.Router.Route.HTTP.Spec.ParentRefs {
		if string(parentRef.Name) != m.gatewayRef.Name {
			continue
		}

		parentNamespace := llmIsvc.Namespace
		if parentRef.Namespace != nil {
			parentNamespace = string(*parentRef.Namespace)
		}

		if parentNamespace == m.gatewayRef.Namespace {
			return true
		}
	}

	return false
}

func (m *Manager) hasReferencedRouteAttachedToGateway(ctx context.Context, llmIsvc *kservev1alpha1.LLMInferenceService) bool {
	if llmIsvc.Spec.Router.Route == nil || llmIsvc.Spec.Router.Route.HTTP == nil || len(llmIsvc.Spec.Router.Route.HTTP.Refs) == 0 {
		return false
	}

	for _, routeRef := range llmIsvc.Spec.Router.Route.HTTP.Refs {
		route, err := m.gatewayClient.HTTPRoutes(llmIsvc.Namespace).Get(ctx, routeRef.Name, metav1.GetOptions{})
		if err != nil {
			log.Printf("DEBUG: Failed to fetch HTTPRoute %s/%s: %v", llmIsvc.Namespace, routeRef.Name, err)
			continue
		}

		for _, parentRef := range route.Spec.ParentRefs {
			if string(parentRef.Name) != m.gatewayRef.Name {
				continue
			}

			parentNamespace := llmIsvc.Namespace
			if parentRef.Namespace != nil {
				parentNamespace = string(*parentRef.Namespace)
			}

			if parentNamespace == m.gatewayRef.Namespace {
				return true
			}
		}
	}

	return false
}

func (m *Manager) hasManagedRouteAttachedToGateway(ctx context.Context, llmIsvc *kservev1alpha1.LLMInferenceService) bool {
	if llmIsvc.Spec.Router.Route == nil || llmIsvc.Spec.Router.Route.HTTP == nil {
		return false
	}

	httpRoute := llmIsvc.Spec.Router.Route.HTTP
	if httpRoute.Spec != nil || len(httpRoute.Refs) > 0 {
		return false
	}

	labelSelector := labels.SelectorFromSet(labels.Set{
		"app.kubernetes.io/component": "llminferenceservice-router",
		"app.kubernetes.io/name":      llmIsvc.Name,
		"app.kubernetes.io/part-of":   "llminferenceservice",
	})

	routes, err := m.gatewayClient.HTTPRoutes(llmIsvc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		log.Printf("DEBUG: Failed to list HTTPRoutes for LLM %s/%s: %v", llmIsvc.Namespace, llmIsvc.Name, err)
		return false
	}

	if len(routes.Items) == 0 {
		return false
	}

	for _, route := range routes.Items {
		for _, parentRef := range route.Spec.ParentRefs {
			if string(parentRef.Name) != m.gatewayRef.Name {
				continue
			}

			parentNamespace := llmIsvc.Namespace
			if parentRef.Namespace != nil {
				parentNamespace = string(*parentRef.Namespace)
			}

			if parentNamespace == m.gatewayRef.Namespace {
				return true
			}
		}
	}

	return false
}
