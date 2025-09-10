package token

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Reviewer handles token validation
type Reviewer struct {
	clientset kubernetes.Interface
}

// NewReviewer creates a new Reviewer instance
func NewReviewer(clientset kubernetes.Interface) *Reviewer {
	return &Reviewer{
		clientset: clientset,
	}
}

// ExtractUserInfo validates a token and extracts user information
func (r *Reviewer) ExtractUserInfo(ctx context.Context, token string) (*UserContext, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	tokenReview := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: token,
		},
	}

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
	}, nil
}
