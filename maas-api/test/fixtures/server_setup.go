package fixtures

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	kservev1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kserveclientv1alpha1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	kservefakev1alpha1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1alpha1/fake"
	kserveclientv1beta1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1beta1"
	kservefakev1beta1 "github.com/kserve/kserve/pkg/client/clientset/versioned/typed/serving/v1beta1/fake"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
)

// TokenReviewScenario defines how TokenReview should respond for a given token.
type TokenReviewScenario struct {
	Authenticated bool
	UserInfo      authv1.UserInfo
	ShouldError   bool
	ErrorMessage  string
}

// TestServerConfig holds configuration for test server setup.
type TestServerConfig struct {
	WithTierConfig bool
	TokenScenarios map[string]TokenReviewScenario
	Objects        []runtime.Object
	TestNamespace  string
	TestTenant     string
}

// TestClients holds the test clients.
type TestClients struct {
	K8sClient      kubernetes.Interface
	KServeV1Beta1  kserveclientv1beta1.ServingV1beta1Interface
	KServeV1Alpha1 kserveclientv1alpha1.ServingV1alpha1Interface
}

// TestComponents holds common test components.
type TestComponents struct {
	Manager   *token.Manager
	Reviewer  *token.Reviewer
	Clientset *k8sfake.Clientset
}

// SetupTestServer creates a test server with base configuration.
func SetupTestServer(_ *testing.T, config TestServerConfig) (*gin.Engine, *TestClients) {
	gin.SetMode(gin.TestMode)

	if config.TestNamespace == "" {
		config.TestNamespace = TestNamespace
	}
	if config.TestTenant == "" {
		config.TestTenant = TestTenant
	}

	// Separate k8s objects from KServe objects
	var k8sObjects []runtime.Object
	var kserveObjects []runtime.Object

	for _, obj := range config.Objects {
		if gvk := obj.GetObjectKind().GroupVersionKind(); gvk.Group == "serving.kserve.io" {
			kserveObjects = append(kserveObjects, obj)
		} else {
			k8sObjects = append(k8sObjects, obj)
		}
	}

	if config.WithTierConfig {
		configMap := CreateTierConfigMap(config.TestNamespace)
		k8sObjects = append(k8sObjects, configMap)
	}

	k8sClient := k8sfake.NewClientset(k8sObjects...)

	if config.TokenScenarios != nil {
		StubTokenReview(k8sClient, config.TokenScenarios)
	}

	scheme := runtime.NewScheme()
	_ = kservev1beta1.AddToScheme(scheme)
	_ = kservev1alpha1.AddToScheme(scheme)

	// Create fake KServe clients with test objects
	fakeKServeClient := k8stesting.Fake{}

	// Create object tracker with the correct codec
	codecFactory := serializer.NewCodecFactory(scheme)
	tracker := k8stesting.NewObjectTracker(scheme, codecFactory.UniversalDecoder())

	// Add KServe objects to the tracker
	for _, obj := range kserveObjects {
		_ = tracker.Add(obj)
	}

	fakeKServeClient.AddReactor("*", "*", k8stesting.ObjectReaction(tracker))

	kserveV1Beta1 := &kservefakev1beta1.FakeServingV1beta1{Fake: &fakeKServeClient}
	kserveV1Alpha1 := &kservefakev1alpha1.FakeServingV1alpha1{Fake: &fakeKServeClient}

	clients := &TestClients{
		K8sClient:      k8sClient,
		KServeV1Beta1:  kserveV1Beta1,
		KServeV1Alpha1: kserveV1Alpha1,
	}

	return gin.New(), clients
}

// StubTokenProviderAPIs creates common test components for token tests.
func StubTokenProviderAPIs(_ *testing.T, withTierConfig bool, tokenScenarios map[string]TokenReviewScenario) (*token.Manager, *token.Reviewer, *k8sfake.Clientset) {
	var objects []runtime.Object

	if withTierConfig {
		configMap := CreateTierConfigMap(TestNamespace)
		objects = append(objects, configMap)
	}

	fakeClient := k8sfake.NewClientset(objects...)

	StubTokenReview(fakeClient, tokenScenarios)

	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	namespaceLister := informerFactory.Core().V1().Namespaces().Lister()
	serviceAccountLister := informerFactory.Core().V1().ServiceAccounts().Lister()

	tierMapper := tier.NewMapper(fakeClient, TestTenant, TestNamespace)
	manager := token.NewManager(
		TestTenant,
		tierMapper,
		fakeClient,
		namespaceLister,
		serviceAccountLister,
	)
	reviewer := token.NewReviewer(fakeClient)

	return manager, reviewer, fakeClient
}

// SetupTestRouter creates a test router with token endpoints.
func SetupTestRouter(manager *token.Manager, reviewer *token.Reviewer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := token.NewHandler("test", manager)

	protected := router.Group("/v1")
	if reviewer != nil {
		protected.Use(token.ExtractUserInfo(reviewer))
	}
	protected.POST("/tokens", handler.IssueToken)
	protected.DELETE("/tokens", handler.RevokeAllTokens)

	return router
}

// SetupTierTestRouter creates a test router for tier endpoints.
func SetupTierTestRouter(mapper *tier.Mapper) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := tier.NewHandler(mapper)
	router.POST("/tiers/lookup", handler.TierLookup)

	return router
}

// CreateTestMapper creates a tier mapper for testing.
func CreateTestMapper(withConfigMap bool) *tier.Mapper {
	var objects []runtime.Object

	if withConfigMap {
		configMap := CreateTierConfigMap(TestNamespace)
		objects = append(objects, configMap)
	}

	clientset := k8sfake.NewClientset(objects...)
	return tier.NewMapper(clientset, TestTenant, TestNamespace)
}

// StubTokenReview sets up TokenReview API mocking for authentication tests.
func StubTokenReview(clientset kubernetes.Interface, scenarios map[string]TokenReviewScenario) {
	fakeClient, ok := clientset.(*k8sfake.Clientset)
	if !ok {
		panic("StubTokenReview: clientset is not a *k8sfake.Clientset")
	}
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(k8stesting.CreateAction)
		if !ok {
			return true, nil, fmt.Errorf("expected CreateAction, got %T", action)
		}
		tokenReview, ok := createAction.GetObject().(*authv1.TokenReview)
		if !ok {
			return true, nil, fmt.Errorf("expected TokenReview, got %T", createAction.GetObject())
		}
		tokenSpec := tokenReview.Spec.Token

		scenario, exists := scenarios[tokenSpec]
		if !exists {
			return true, &authv1.TokenReview{
				Status: authv1.TokenReviewStatus{
					Authenticated: false,
				},
			}, nil
		}

		if scenario.ShouldError {
			return true, nil, fmt.Errorf("tokenSpec review API error: %s", scenario.ErrorMessage)
		}

		tokenReview.Status = authv1.TokenReviewStatus{
			Authenticated: scenario.Authenticated,
			User:          scenario.UserInfo,
		}

		return true, tokenReview, nil
	})

	fakeClient.PrependReactor("create", "serviceaccounts/token", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(k8stesting.CreateAction)
		if !ok {
			return true, nil, fmt.Errorf("expected CreateAction, got %T", action)
		}
		tokenRequest, ok := createAction.GetObject().(*authv1.TokenRequest)
		if !ok {
			return true, nil, fmt.Errorf("expected TokenRequest, got %T", createAction.GetObject())
		}

		tokenRequest.Status = authv1.TokenRequestStatus{
			Token:               "mock-service-account-token-" + strconv.FormatInt(time.Now().Unix(), 10),
			ExpirationTimestamp: metav1.NewTime(time.Now().Add(time.Hour)),
		}

		return true, tokenRequest, nil
	})
}
