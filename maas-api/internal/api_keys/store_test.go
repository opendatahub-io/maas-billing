package api_keys_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/api_keys"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	// Create a temporary directory for the database using t.TempDir()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Test NewStore
	store, err := api_keys.NewStore(dbPath)
	if err == nil && store != nil {
		defer store.Close()
	}
	require.NoError(t, err)

	ctx := t.Context()

	t.Run("AddTokenMetadata", func(t *testing.T) {
		tok := &token.Token{
			Name:      "token1",
			JTI:       "jti1",
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", tok)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		require.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token1", tokens[0].Name)
		assert.Equal(t, "active", tokens[0].Status)
	})

	t.Run("AddSecondToken", func(t *testing.T) {
		tok := &token.Token{
			Name:      "token2",
			JTI:       "jti2",
			ExpiresAt: time.Now().Add(2 * time.Hour).Unix(),
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", tok)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		require.NoError(t, err)
		assert.Len(t, tokens, 2)
	})

	t.Run("GetTokensForDifferentUser", func(t *testing.T) {
		tok := &token.Token{
			Name:      "token3",
			JTI:       "jti3",
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user2", tok)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user2")
		require.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token3", tokens[0].Name)
	})

	t.Run("DeleteTokensForUser", func(t *testing.T) {
		err := store.DeleteTokensForUser(ctx, "test-ns", "user1")
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		require.NoError(t, err)
		assert.Empty(t, tokens)

		// User2 should still exist
		tokens2, err := store.GetTokensForUser(ctx, "user2")
		require.NoError(t, err)
		assert.Len(t, tokens2, 1)
	})

	t.Run("GetToken", func(t *testing.T) {
		// Retrieve user2's token by JTI
		gotToken, err := store.GetToken(ctx, "user2", "jti3")
		require.NoError(t, err)
		assert.NotNil(t, gotToken)
		assert.Equal(t, "token3", gotToken.Name)
	})

	t.Run("ExpiredTokenStatus", func(t *testing.T) {
		// Add an expired token
		tok := &token.Token{
			Name:      "expired-token",
			JTI:       "jti-expired",
			ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
		}
		err := store.AddTokenMetadata(ctx, "test-ns", "user4", tok)
		require.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user4")
		require.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "expired", tokens[0].Status)

		// Get single token check
		gotToken, err := store.GetToken(ctx, "user4", "jti-expired")
		require.NoError(t, err)
		assert.Equal(t, "expired", gotToken.Status)
	})
}

