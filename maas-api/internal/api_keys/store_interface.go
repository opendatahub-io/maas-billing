package api_keys

import (
	"context"
	"errors"
)

var ErrTokenNotFound = errors.New("token not found")

const (
	TokenStatusActive = "active"
	TokenStatusExpired = "expired"
)

type Store interface {
	AddTokenMetadata(ctx context.Context, namespace, username string, apiKey *APIKey) error

	GetTokensForUser(ctx context.Context, namespace, username string) ([]ApiKeyMetadata, error)

	GetToken(ctx context.Context, namespace, username, jti string) (*ApiKeyMetadata, error)

	MarkTokensAsExpiredForUser(ctx context.Context, namespace, username string) error

	Close() error
}

