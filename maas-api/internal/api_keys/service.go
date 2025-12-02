package api_keys

import (
	"context"
	"fmt"
	"time"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
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

func (s *Service) CreateAPIKey(ctx context.Context, user *token.UserContext, name string, expiration time.Duration) (*token.Token, error) {
	// Generate token (name is set after generation since persistence is handled here)
	tok, err := s.tokenManager.GenerateToken(ctx, user, expiration, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	tok.Name = name

	if err := s.store.AddTokenMetadata(ctx, tok.Namespace, user.Username, tok); err != nil {
		return nil, fmt.Errorf("failed to persist api key metadata: %w", err)
	}

	return tok, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, user *token.UserContext) ([]NamedToken, error) {
	namespace, err := s.tokenManager.GetNamespaceForUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to determine namespace for user: %w", err)
	}
	return s.store.GetTokensForUser(ctx, namespace, user.Username)
}

func (s *Service) GetAPIKey(ctx context.Context, user *token.UserContext, id string) (*NamedToken, error) {
	namespace, err := s.tokenManager.GetNamespaceForUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to determine namespace for user: %w", err)
	}
	return s.store.GetToken(ctx, namespace, user.Username, id)
}

func (s *Service) RevokeAPIKey(ctx context.Context, user *token.UserContext, id string) error {
	namespace, err := s.tokenManager.GetNamespaceForUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to determine namespace for user: %w", err)
	}
	return s.store.DeleteToken(ctx, namespace, user.Username, id)
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

