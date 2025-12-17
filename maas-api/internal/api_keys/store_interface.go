package api_keys

import (
	"context"
	"errors"
)

var ErrTokenNotFound = errors.New("token not found")

const (
	TokenStatusActive  = "active"
	TokenStatusExpired = "expired"
)

type MetadataStore interface {
	Add(ctx context.Context, username string, apiKey *APIKey) error

	List(ctx context.Context, username string) ([]ApiKeyMetadata, error)

	Get(ctx context.Context, jti string) (*ApiKeyMetadata, error)

	// InvalidateAll marks all active tokens for a user as expired.
	InvalidateAll(ctx context.Context, username string) error

	Close() error
}
