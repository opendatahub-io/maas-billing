package api_keys

import (
	"context"
	"fmt"
	"time"

	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/token"
)

type TokenManager interface {
	GenerateToken(ctx context.Context, user *token.UserContext, expiration time.Duration, name string) (*token.Token, error)
	RevokeTokens(ctx context.Context, user *token.UserContext) (string, error)
	// GetNamespaceForUser returns the namespace for a user based on their tier
	GetNamespaceForUser(ctx context.Context, user *token.UserContext) (string, error)
}

type Service struct {
	tokenManager TokenManager
	store        *Store
}

func NewService(tokenManager TokenManager, store *Store) *Service {
	return &Service{
		tokenManager: tokenManager,
		store:        store,
	}
}

func (s *Service) CreateAPIKey(ctx context.Context, user *token.UserContext, name string, description string, expiration time.Duration) (*APIKey, error) {
	// Generate token
	tok, err := s.tokenManager.GenerateToken(ctx, user, expiration, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Get namespace for the user
	namespace, err := s.tokenManager.GetNamespaceForUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to determine namespace for user: %w", err)
	}

	// Create APIKey with embedded Token and metadata
	apiKey := &APIKey{
		Token:       *tok,
		Name:        name,
		Description: description,
		Namespace:   namespace,
	}

	if err := s.store.AddTokenMetadata(ctx, namespace, user.Username, apiKey); err != nil {
		return nil, fmt.Errorf("failed to persist api key metadata: %w", err)
	}

	return apiKey, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, user *token.UserContext) ([]ApiKeyMetadata, error) {
	namespace, err := s.tokenManager.GetNamespaceForUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to determine namespace for user: %w", err)
	}
	return s.store.GetTokensForUser(ctx, namespace, user.Username)
}

func (s *Service) GetAPIKey(ctx context.Context, user *token.UserContext, id string) (*ApiKeyMetadata, error) {
	namespace, err := s.tokenManager.GetNamespaceForUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to determine namespace for user: %w", err)
	}
	return s.store.GetToken(ctx, namespace, user.Username, id)
}

// RevokeAll invalidates all tokens for the user (ephemeral and persistent).
// It recreates the Service Account (invalidating all tokens) and marks API key metadata as expired.
func (s *Service) RevokeAll(ctx context.Context, user *token.UserContext) error {
	// Revoke in K8s (recreate SA) - this invalidates all tokens
	namespace, err := s.tokenManager.RevokeTokens(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to revoke tokens in k8s: %w", err)
	}

	// Mark API key metadata as expired (preserves history)
	if err := s.store.MarkTokensAsExpiredForUser(ctx, namespace, user.Username); err != nil {
		return fmt.Errorf("tokens revoked but failed to mark metadata as expired: %w", err)
	}

	return nil
}
