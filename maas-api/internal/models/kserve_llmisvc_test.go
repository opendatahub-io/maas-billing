package models_test

import (
	"testing"

	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
)

func TestListAvailableLLMs(t *testing.T) {
	gateway := models.GatewayRef{Name: "maas-gateway", Namespace: "gateway-ns"}

	tests := []struct {
		name        string
		llmServices []*kservev1alpha1.LLMInferenceService
		httpRoutes  []*gwapiv1.HTTPRoute
		expectMatch []string
	}{
		{
			name: "direct gateway reference",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-direct", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Gateway: &kservev1alpha1.GatewaySpec{
								Refs: []kservev1alpha1.UntypedObjectReference{
									{Name: "maas-gateway", Namespace: "gateway-ns"},
								},
							},
						},
					},
				},
			},
			expectMatch: []string{"llm-direct"},
		},
		{
			name: "inline HTTPRoute spec ",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-inline", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Route: &kservev1alpha1.GatewayRoutesSpec{
								HTTP: &kservev1alpha1.HTTPRouteSpec{
									Spec: &gwapiv1.HTTPRouteSpec{
										CommonRouteSpec: gwapiv1.CommonRouteSpec{
											ParentRefs: []gwapiv1.ParentReference{
												{Name: "maas-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectMatch: []string{"llm-inline"},
		},
		{
			name: "referenced HTTPRoute",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-ref", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Route: &kservev1alpha1.GatewayRoutesSpec{
								HTTP: &kservev1alpha1.HTTPRouteSpec{
									Refs: []corev1.LocalObjectReference{{Name: "my-route"}},
								},
							},
						},
					},
				},
			},
			httpRoutes: []*gwapiv1.HTTPRoute{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "my-route", Namespace: "test-ns"},
					Spec: gwapiv1.HTTPRouteSpec{
						CommonRouteSpec: gwapiv1.CommonRouteSpec{
							ParentRefs: []gwapiv1.ParentReference{
								{Name: "maas-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
							},
						},
					},
				},
			},
			expectMatch: []string{"llm-ref"},
		},
		{
			name: "managed HTTPRoute",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-managed", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Route: &kservev1alpha1.GatewayRoutesSpec{
								HTTP: &kservev1alpha1.HTTPRouteSpec{},
							},
						},
					},
				},
			},
			httpRoutes: []*gwapiv1.HTTPRoute{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managed-route",
						Namespace: "test-ns",
						Labels: map[string]string{
							"app.kubernetes.io/component": "llminferenceservice-router",
							"app.kubernetes.io/name":      "llm-managed",
							"app.kubernetes.io/part-of":   "llminferenceservice",
						},
					},
					Spec: gwapiv1.HTTPRouteSpec{
						CommonRouteSpec: gwapiv1.CommonRouteSpec{
							ParentRefs: []gwapiv1.ParentReference{
								{Name: "maas-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
							},
						},
					},
				},
			},
			expectMatch: []string{"llm-managed"},
		},
		{
			name: "multiple gateway references with maas-gateway",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-multi-gw", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Gateway: &kservev1alpha1.GatewaySpec{
								Refs: []kservev1alpha1.UntypedObjectReference{
									{Name: "first-gateway", Namespace: "gateway-ns"},
									{Name: "second-gateway", Namespace: "gateway-ns"},
									{Name: "maas-gateway", Namespace: "gateway-ns"},
								},
							},
						},
					},
				},
			},
			expectMatch: []string{"llm-multi-gw"},
		},
		{
			name: "inline HTTPRoute with multiple parent refs and maas-gateway",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-inline-multi", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Route: &kservev1alpha1.GatewayRoutesSpec{
								HTTP: &kservev1alpha1.HTTPRouteSpec{
									Spec: &gwapiv1.HTTPRouteSpec{
										CommonRouteSpec: gwapiv1.CommonRouteSpec{
											ParentRefs: []gwapiv1.ParentReference{
												{Name: "first-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
												{Name: "second-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
												{Name: "maas-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectMatch: []string{"llm-inline-multi"},
		},
		{
			name: "no match different gateway",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-different", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Gateway: &kservev1alpha1.GatewaySpec{
								Refs: []kservev1alpha1.UntypedObjectReference{
									{Name: "other-gateway", Namespace: "gateway-ns"},
								},
							},
						},
					},
				},
			},
			expectMatch: []string{},
		},
		{
			name: "no match inline HTTPRoute with different gateway",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-inline-different", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Route: &kservev1alpha1.GatewayRoutesSpec{
								HTTP: &kservev1alpha1.HTTPRouteSpec{
									Spec: &gwapiv1.HTTPRouteSpec{
										CommonRouteSpec: gwapiv1.CommonRouteSpec{
											ParentRefs: []gwapiv1.ParentReference{
												{Name: "other-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectMatch: []string{},
		},
		{
			name: "no match referenced HTTPRoute with different gateway",
			llmServices: []*kservev1alpha1.LLMInferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "llm-ref-different", Namespace: "test-ns"},
					Spec: kservev1alpha1.LLMInferenceServiceSpec{
						Router: &kservev1alpha1.RouterSpec{
							Route: &kservev1alpha1.GatewayRoutesSpec{
								HTTP: &kservev1alpha1.HTTPRouteSpec{
									Refs: []corev1.LocalObjectReference{{Name: "different-route"}},
								},
							},
						},
					},
				},
			},
			httpRoutes: []*gwapiv1.HTTPRoute{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "different-route", Namespace: "test-ns"},
					Spec: gwapiv1.HTTPRouteSpec{
						CommonRouteSpec: gwapiv1.CommonRouteSpec{
							ParentRefs: []gwapiv1.ParentReference{
								{Name: "other-gateway", Namespace: ptrTo(gwapiv1.Namespace("gateway-ns"))},
							},
						},
					},
				},
			},
			expectMatch: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, errMgr := models.NewManager(
				fixtures.NewInferenceServiceLister(),
				fixtures.NewLLMInferenceServiceLister(fixtures.ToRuntimeObjects(tt.llmServices)...),
				fixtures.NewHTTPRouteLister(fixtures.ToRuntimeObjects(tt.httpRoutes)...),
				gateway,
			)
			require.NoError(t, errMgr)

			availableModels, err := manager.ListAvailableLLMs()
			require.NoError(t, err)

			var actualNames []string
			for _, model := range availableModels {
				actualNames = append(actualNames, model.ID)
			}

			assert.ElementsMatch(t, tt.expectMatch, actualNames)
		})
	}
}

func ptrTo[T any](v T) *T {
	return &v
}
