package token

import (
	"context"
	"errors"
	"fmt"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Reviewer handles token validation.
type Reviewer struct {
	clientset kubernetes.Interface
	audience  string // Optional audience for TokenReview
}

// NewReviewer creates a new Reviewer instance.
// The audience parameter is optional. When provided, it's used to validate Service Account tokens
// that were created with a specific audience (e.g., "{instance}-sa"). The reviewer will first
// attempt validation with the specified audience, then fall back to default audience validation
// for User/OIDC tokens.
func NewReviewer(clientset kubernetes.Interface, audience ...string) *Reviewer {
	r := &Reviewer{
		clientset: clientset,
	}
	if len(audience) > 0 && audience[0] != "" {
		r.audience = audience[0]
	}
	return r
}

// ExtractUserInfo validates a token and extracts user information.
func (r *Reviewer) ExtractUserInfo(ctx context.Context, token string) (*UserContext, error) {
	if token == "" {
		return nil, errors.New("token cannot be empty")
	}

	claims, err := extractClaims(token)
	if err != nil {
		// Log the error but don't fail the request, as jti is only for metadata
		fmt.Printf("Warning: could not extract jti from token: %v\n", err)
	}

	var jti string
	if claims != nil {
		jti, _ = claims["jti"].(string)
	}

	tokenReview := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: token,
		},
	}

	// 1. Try validating with the specific audience if configured (for Service Account tokens)
	if r.audience != "" {
		tokenReview.Spec.Audiences = []string{r.audience}
		result, err := r.clientset.AuthenticationV1().TokenReviews().Create(ctx, tokenReview, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("token review with audience failed: %w", err)
		}

		if result.Status.Authenticated {
			userInfo := result.Status.User
			return &UserContext{
				Username:        userInfo.Username,
				UID:             userInfo.UID,
				Groups:          userInfo.Groups,
				IsAuthenticated: true,
				JTI:             jti,
			}, nil
		}
	}

	// 2. Fallback: Validate without audience (for User tokens / OIDC tokens)
	// Reset audiences to empty to validate against default audience
	tokenReview.Spec.Audiences = nil
	result, err := r.clientset.AuthenticationV1().TokenReviews().Create(ctx, tokenReview, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("token review failed: %w", err)
	}

	if !result.Status.Authenticated {
		return &UserContext{
			IsAuthenticated: false,
		}, nil
	}

	userInfo := result.Status.User
	return &UserContext{
		Username:        userInfo.Username,
		UID:             userInfo.UID,
		Groups:          userInfo.Groups,
		IsAuthenticated: true,
		JTI:             jti,
	}, nil
}
