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
	// Generate the token. We pass "" as name because persistence is handled here.
	tok, err := s.tokenManager.GenerateToken(ctx, user, expiration, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	tok.Name = name

	// Store metadata
	if err := s.store.AddTokenMetadata(ctx, tok.Namespace, user.Username, tok); err != nil {
		return nil, fmt.Errorf("failed to persist api key metadata: %w", err)
	}

	return tok, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, user *token.UserContext) ([]NamedToken, error) {
	return s.store.GetTokensForUser(ctx, user.Username)
}

func (s *Service) GetAPIKey(ctx context.Context, user *token.UserContext, id string) (*NamedToken, error) {
	return s.store.GetToken(ctx, user.Username, id)
}

func (s *Service) RevokeAPIKey(ctx context.Context, user *token.UserContext, id string) error {
	return s.store.DeleteToken(ctx, user.Username, id)
}

// RevokeAll invalidates all tokens for the user (ephemeral and persistent)
func (s *Service) RevokeAll(ctx context.Context, user *token.UserContext) error {
	// Revoke in K8s (recreate SA)
	namespace, err := s.tokenManager.RevokeTokens(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to revoke tokens in k8s: %w", err)
	}

	// Clean up metadata
	if err := s.store.DeleteTokensForUser(ctx, namespace, user.Username); err != nil {
		// We log error but don't fail the whole operation as the critical part (SA recreation) succeeded
		// Ideally we should log this. returning error might be okay too.
		return fmt.Errorf("tokens revoked but failed to clear metadata: %w", err)
	}

	return nil
}

