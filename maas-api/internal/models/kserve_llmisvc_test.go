package models_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	kservev1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kservefakev1alpha1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1alpha1/fake"
	kservefakev1beta1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1beta1/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8stesting "k8s.io/client-go/testing"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayfake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1/fake"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/models"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
)

const (
	testGatewayName      = "maas-gateway"
	testGatewayNamespace = "gateway-ns"
)

func TestListAvailableLLMs(t *testing.T) { //nolint:maintidx // linter is complaining about the test cases being in a different order than the expected match
	gateway := models.GatewayRef{Name: testGatewayName, Namespace: testGatewayNamespace}

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
			fakeKServe := &k8stesting.Fake{}
			fakeKServe.AddReactor("list", "llminferenceservices", func(action k8stesting.Action) (bool, runtime.Object, error) {
				list := &kservev1alpha1.LLMInferenceServiceList{Items: []kservev1alpha1.LLMInferenceService{}}
				for _, llm := range tt.llmServices {
					list.Items = append(list.Items, *llm)
				}
				return true, list, nil
			})

			fakeGateway := &k8stesting.Fake{}
			fakeGateway.AddReactor("get", "httproutes", func(action k8stesting.Action) (bool, runtime.Object, error) {
				getAction, ok := action.(k8stesting.GetAction)
				if !ok {
					return false, nil, fmt.Errorf("invalid action: %T", action)
				}
				for _, route := range tt.httpRoutes {
					if route.Name == getAction.GetName() && route.Namespace == getAction.GetNamespace() {
						return true, route, nil
					}
				}

				return true, nil, fmt.Errorf("httproutes.gateway.networking.k8s.io %q not found", getAction.GetName())
			})
			fakeGateway.AddReactor("list", "httproutes", func(action k8stesting.Action) (bool, runtime.Object, error) {
				list := &gwapiv1.HTTPRouteList{Items: []gwapiv1.HTTPRoute{}}
				for _, route := range tt.httpRoutes {
					list.Items = append(list.Items, *route)
				}
				return true, list, nil
			})

			manager := models.NewManager(
				&kservefakev1beta1.FakeServingV1beta1{Fake: fakeKServe},
				&kservefakev1alpha1.FakeServingV1alpha1{Fake: fakeKServe},
				&gatewayfake.FakeGatewayV1{Fake: fakeGateway},
				nil, // k8sClient not needed for this test
				gateway,
			)

			availableModels, err := manager.ListAvailableLLMs(t.Context())
			require.NoError(t, err)

			var actualNames []string
			for _, model := range availableModels {
				actualNames = append(actualNames, model.ID)
			}

			assert.ElementsMatch(t, tt.expectMatch, actualNames)
		})
	}
}

func TestListAvailableLLMsForUser(t *testing.T) {
	// Test scenarios for gateway delegation authorization
	tests := []struct {
		name          string
		user          *token.UserContext
		httpResponses map[string]int // model URL path -> HTTP status code
		expectModels  []string       // expected model IDs
		expectError   bool
	}{
		{
			name: "user with full access",
			user: &token.UserContext{
				Username: "test-user",
				Groups:   []string{"test-group"},
			},
			httpResponses: map[string]int{
				"/llama-7b/health": http.StatusOK,
				"/gpt-3/health":    http.StatusOK,
			},
			expectModels: []string{"llama-7b", "gpt-3"},
		},
		{
			name: "user with no access",
			user: &token.UserContext{
				Username: "restricted-user",
				Groups:   []string{"restricted-group"},
			},
			httpResponses: map[string]int{
				"/llama-7b/health": http.StatusForbidden,
				"/gpt-3/health":    http.StatusUnauthorized,
			},
			expectModels: []string{}, // no models should be returned
		},
		{
			name: "user with partial access",
			user: &token.UserContext{
				Username: "partial-user",
				Groups:   []string{"partial-group"},
			},
			httpResponses: map[string]int{
				"/llama-7b/health": http.StatusOK,
				"/gpt-3/health":    http.StatusForbidden,
			},
			expectModels: []string{"llama-7b"}, // only allowed model
		},
		{
			name: "gateway server error",
			user: &token.UserContext{
				Username: "error-user",
				Groups:   []string{"error-group"},
			},
			httpResponses: map[string]int{
				"/llama-7b/health": http.StatusInternalServerError,
				"/gpt-3/health":    http.StatusServiceUnavailable,
			},
			expectModels: []string{}, // no models on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server to simulate gateway responses
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check for Authorization header
				auth := r.Header.Get("Authorization")
				if !strings.HasPrefix(auth, "Bearer test-token") {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Return configured response for this path
				if statusCode, exists := tt.httpResponses[r.URL.Path]; exists {
					w.WriteHeader(statusCode)
					return
				}

				// Default to 404 for unknown paths
				w.WriteHeader(http.StatusNotFound)
			}))
			defer mockServer.Close()

			// Create test LLM services with URLs pointing to our mock server
			llmTestScenarios := []fixtures.LLMTestScenario{
				{
					Name:             "llm1",
					Namespace:        "test-ns",
					SpecModelName:    strPtr("llama-7b"),
					URL:              mustParseURL(mockServer.URL + "/llama-7b"),
					GatewayName:      testGatewayName,
					GatewayNamespace: testGatewayNamespace,
					Ready:            true,
				},
				{
					Name:             "llm2",
					Namespace:        "test-ns",
					SpecModelName:    strPtr("gpt-3"),
					URL:              mustParseURL(mockServer.URL + "/gpt-3"),
					GatewayName:      testGatewayName,
					GatewayNamespace: testGatewayNamespace,
					Ready:            true,
				},
			}
			llmInferenceServices := fixtures.CreateLLMInferenceServices(llmTestScenarios...)

			// Create test objects
			var objects []runtime.Object
			objects = append(objects, llmInferenceServices...)

			// Setup clients
			scheme := runtime.NewScheme()
			_ = kservev1beta1.AddToScheme(scheme)
			_ = kservev1alpha1.AddToScheme(scheme)

			// Create fake KServe clients
			fakeKServe := k8stesting.Fake{}
			codecFactory := serializer.NewCodecFactory(scheme)
			tracker := k8stesting.NewObjectTracker(scheme, codecFactory.UniversalDecoder())
			for _, obj := range objects {
				_ = tracker.Add(obj)
			}
			fakeKServe.AddReactor("*", "*", k8stesting.ObjectReaction(tracker))

			// Create manager (k8sClient not needed for gateway delegation)
			gateway := models.GatewayRef{
				Name:      testGatewayName,
				Namespace: testGatewayNamespace,
			}

			manager := models.NewManager(
				&kservefakev1beta1.FakeServingV1beta1{Fake: &fakeKServe},
				&kservefakev1alpha1.FakeServingV1alpha1{Fake: &fakeKServe},
				&gatewayfake.FakeGatewayV1{Fake: &k8stesting.Fake{}},
				nil, // k8sClient not used anymore
				gateway,
			)

			// Test the user-aware filtering with SA token
			availableModels, err := manager.ListAvailableLLMsForUser(context.Background(), tt.user, "test-token")

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Extract model IDs
			var actualModelIDs []string
			for _, model := range availableModels {
				actualModelIDs = append(actualModelIDs, model.ID)
			}

			assert.ElementsMatch(t, tt.expectModels, actualModelIDs)
		})
	}
}

// Helper function for test.
func mustParseURL(rawURL string) fixtures.PublicURL {
	return fixtures.PublicURL(rawURL)
}

func ptrTo[T any](v T) *T {
	return &v
}

func strPtr(s string) *string {
	return &s
}
