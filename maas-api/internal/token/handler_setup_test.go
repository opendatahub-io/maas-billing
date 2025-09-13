package token_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
	"github.com/opendatahub-io/maas-billing/maas-api/test/fixtures"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// tokenReviewScenario defines how TokenReview should respond for a given token
type tokenReviewScenario struct {
	Authenticated bool
	UserInfo      authv1.UserInfo
	ShouldError   bool
	ErrorMessage  string
}

// stubTokenReview sets up TokenReview API mocking for authentication tests
func stubTokenReview(clientset *fake.Clientset, scenarios map[string]tokenReviewScenario) {
	clientset.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(k8stesting.CreateAction)
		tokenReview := createAction.GetObject().(*authv1.TokenReview)
		t := tokenReview.Spec.Token

		scenario, exists := scenarios[t]
		if !exists {
			return false, nil, fmt.Errorf("no scenario for token '%s'", t)
		}

		if scenario.ShouldError {
			return true, nil, fmt.Errorf("token review API error: %s", scenario.ErrorMessage)
		}

		tokenReview.Status = authv1.TokenReviewStatus{
			Authenticated: scenario.Authenticated,
			User:          scenario.UserInfo,
		}

		return true, tokenReview, nil
	})
}

func createTestComponents(t *testing.T, withTierConfig bool, tokenScenarios map[string]tokenReviewScenario) (*token.Manager, *token.Reviewer, *fake.Clientset) {
	var objects []runtime.Object

	if withTierConfig {
		configMap := fixtures.CreateTierConfigMap(testNamespace)
		objects = append(objects, configMap)
	}

	clientset := fake.NewSimpleClientset(objects...)

	clientset.PrependReactor("create", "serviceaccounts/token", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(k8stesting.CreateAction)
		tokenRequest := createAction.GetObject().(*authv1.TokenRequest)

		tokenRequest.Status = authv1.TokenRequestStatus{
			Token:               "mock-service-account-token-" + fmt.Sprintf("%d", time.Now().Unix()),
			ExpirationTimestamp: metav1.NewTime(time.Now().Add(time.Hour)),
		}

		return true, tokenRequest, nil
	})

	stubTokenReview(clientset, tokenScenarios)

	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	namespaceLister := informerFactory.Core().V1().Namespaces().Lister()
	serviceAccountLister := informerFactory.Core().V1().ServiceAccounts().Lister()

	tierMapper := tier.NewMapper(clientset, testTenant, testNamespace)
	manager := token.NewManager(
		testTenant,
		tierMapper,
		clientset,
		namespaceLister,
		serviceAccountLister,
	)
	reviewer := token.NewReviewer(clientset)

	return manager, reviewer, clientset
}

func setupTestRouter(manager *token.Manager, reviewer *token.Reviewer) *gin.Engine {
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
