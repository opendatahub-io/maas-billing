package token_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
)

func TestResolver_ExtractUserInfo(t *testing.T) {
	tests := []struct {
		name                string
		token               string
		tokenReviewResponse *authv1.TokenReview
		tokenReviewError    error
		expectedResult      *token.UserContext
		expectedError       string
	}{
		{
			name:          "empty token",
			token:         "",
			expectedError: "token cannot be empty",
		},
		{
			name:             "token review API error",
			token:            "valid-token",
			tokenReviewError: errors.New("API error"),
			expectedError:    "token review failed: API error",
		},
		{
			name:  "invalid token - not authenticated",
			token: "invalid-token",
			tokenReviewResponse: &authv1.TokenReview{
				Status: authv1.TokenReviewStatus{
					Authenticated: false,
				},
			},
			expectedResult: &token.UserContext{
				IsAuthenticated: false,
			},
		},
		{
			name:  "valid token with maas groups",
			token: "valid-token",
			tokenReviewResponse: &authv1.TokenReview{
				Status: authv1.TokenReviewStatus{
					Authenticated: true,
					User: authv1.UserInfo{
						Username: "maas-user",
						UID:      "user123",
						Groups:   []string{"system:authenticated", "maas-free-user", "maas-admin"},
					},
				},
			},
			expectedResult: &token.UserContext{
				Username:        "maas-user",
				UID:             "user123",
				Groups:          []string{"system:authenticated", "maas-free-user", "maas-admin"},
				IsAuthenticated: true,
			},
		},
		{
			name:  "valid token with no groups",
			token: "valid-token",
			tokenReviewResponse: &authv1.TokenReview{
				Status: authv1.TokenReviewStatus{
					Authenticated: true,
					User: authv1.UserInfo{
						Username: "minimal-user",
						UID:      "user456",
						Groups:   []string{},
					},
				},
			},
			expectedResult: &token.UserContext{
				Username:        "minimal-user",
				UID:             "user456",
				Groups:          []string{},
				IsAuthenticated: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()

			if tt.tokenReviewResponse != nil || tt.tokenReviewError != nil {
				fakeClient.PrependReactor("create", "tokenreviews", func(action ktesting.Action) (bool, runtime.Object, error) {
					if tt.tokenReviewError != nil {
						return true, nil, tt.tokenReviewError
					}
					return true, tt.tokenReviewResponse, nil
				})
			}

			resolver := token.NewReviewer(fakeClient)
			ctx := context.Background()

			result, err := resolver.ExtractUserInfo(ctx, tt.token)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !userContextEqual(result, tt.expectedResult) {
				t.Errorf("Expected result %+v, got %+v", tt.expectedResult, result)
			}
		})
	}
}

func userContextEqual(a, b *token.UserContext) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Username == b.Username &&
		a.UID == b.UID &&
		a.IsAuthenticated == b.IsAuthenticated &&
		slices.Equal(a.Groups, b.Groups)
}
