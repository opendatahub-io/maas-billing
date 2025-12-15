package api_keys_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/api_keys"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/logger"
	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	// Create a temporary directory for the database using t.TempDir()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	ctx := t.Context()

	// Test NewStore
	testLogger := logger.Development()
	store, err := api_keys.NewStore(ctx, testLogger, dbPath)
	if err == nil && store != nil {
		defer store.Close()
	}
	require.NoError(t, err)

	t.Run("AddTokenMetadata", func(t *testing.T) {
		apiKey := &api_keys.APIKey{
			Token: token.Token{
				JTI:       "jti1",
				ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			},
			Name: "token1",
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", apiKey)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "test-ns", "user1")
		require.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token1", tokens[0].Name)
		assert.Equal(t, api_keys.TokenStatusActive, tokens[0].Status)
	})

	t.Run("AddSecondToken", func(t *testing.T) {
		apiKey := &api_keys.APIKey{
			Token: token.Token{
				JTI:       "jti2",
				ExpiresAt: time.Now().Add(2 * time.Hour).Unix(),
			},
			Name: "token2",
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", apiKey)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "test-ns", "user1")
		require.NoError(t, err)
		assert.Len(t, tokens, 2)
	})

	t.Run("GetTokensForDifferentUser", func(t *testing.T) {
		apiKey := &api_keys.APIKey{
			Token: token.Token{
				JTI:       "jti3",
				ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			},
			Name: "token3",
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user2", apiKey)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "test-ns", "user2")
		require.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token3", tokens[0].Name)
	})

	t.Run("MarkTokensAsExpiredForUser", func(t *testing.T) {
		err := store.MarkTokensAsExpiredForUser(ctx, "test-ns", "user1")
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "test-ns", "user1")
		require.NoError(t, err)
		// Tokens should still exist but marked expired status
		assert.Len(t, tokens, 2)
		for _, tok := range tokens {
			assert.Equal(t, api_keys.TokenStatusExpired, tok.Status)
		}

		// User2 should still exist
		tokens2, err := store.GetTokensForUser(ctx, "test-ns", "user2")
		require.NoError(t, err)
		assert.Len(t, tokens2, 1)
	})

	t.Run("GetToken", func(t *testing.T) {
		// Retrieve user2's token by JTI
		gotToken, err := store.GetToken(ctx, "test-ns", "user2", "jti3")
		require.NoError(t, err)
		assert.NotNil(t, gotToken)
		assert.Equal(t, "token3", gotToken.Name)
	})

	t.Run("ExpiredTokenStatus", func(t *testing.T) {
		// Add an expired token
		apiKey := &api_keys.APIKey{
			Token: token.Token{
				JTI:       "jti-expired",
				ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
			},
			Name: "expired-token",
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user4", apiKey)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "test-ns", "user4")
		require.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, api_keys.TokenStatusExpired, tokens[0].Status)

		// Get single token check
		gotToken, err := store.GetToken(ctx, "test-ns", "user4", "jti-expired")
		require.NoError(t, err)
		assert.Equal(t, api_keys.TokenStatusExpired, gotToken.Status)
	})

	t.Run("CrossNamespaceIsolation", func(t *testing.T) {
		// Create tokens for the same username in two different namespaces
		apiKey1 := &api_keys.APIKey{
			Token: token.Token{
				JTI:       "jti-ns1",
				ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			},
			Name: "ns1-token",
		}
		err := store.AddTokenMetadata(ctx, "namespace-1", "shared-user", apiKey1)
		require.NoError(t, err)

		apiKey2 := &api_keys.APIKey{
			Token: token.Token{
				JTI:       "jti-ns2",
				ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			},
			Name: "ns2-token",
		}
		err = store.AddTokenMetadata(ctx, "namespace-2", "shared-user", apiKey2)
		require.NoError(t, err)

		// Verify namespace-1 only returns tokens from namespace-1
		tokens1, err := store.GetTokensForUser(ctx, "namespace-1", "shared-user")
		require.NoError(t, err)
		assert.Len(t, tokens1, 1)
		assert.Equal(t, "ns1-token", tokens1[0].Name)
		assert.Equal(t, "jti-ns1", tokens1[0].ID)

		// Verify namespace-2 only returns tokens from namespace-2
		tokens2, err := store.GetTokensForUser(ctx, "namespace-2", "shared-user")
		require.NoError(t, err)
		assert.Len(t, tokens2, 1)
		assert.Equal(t, "ns2-token", tokens2[0].Name)
		assert.Equal(t, "jti-ns2", tokens2[0].ID)

		// Verify GetToken respects namespace
		gotToken1, err := store.GetToken(ctx, "namespace-1", "shared-user", "jti-ns1")
		require.NoError(t, err)
		assert.Equal(t, "ns1-token", gotToken1.Name)

		// Verify token from different namespace is not found
		_, err = store.GetToken(ctx, "namespace-1", "shared-user", "jti-ns2")
		require.Error(t, err)
		assert.Equal(t, api_keys.ErrTokenNotFound, err)
	})
}
