package token

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "maas-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Test NewStore
	store, err := NewStore(dbPath)
	assert.NoError(t, err)
	defer store.Close()

	ctx := t.Context()

	t.Run("AddTokenMetadata", func(t *testing.T) {
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", "token1", "hash1", time.Now().Add(1*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		assert.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token1", tokens[0].Name)
		assert.Equal(t, "active", tokens[0].Status)

		// Check status
		active, err := store.IsTokenActive(ctx, "hash1")
		assert.NoError(t, err)
		assert.True(t, active)
	})

	t.Run("AddSecondToken", func(t *testing.T) {
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", "token2", "hash2", time.Now().Add(2*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		assert.NoError(t, err)
		assert.Len(t, tokens, 2)
	})

	t.Run("GetTokensForDifferentUser", func(t *testing.T) {
		err := store.AddTokenMetadata(ctx, "test-ns", "user2", "token3", "hash3", time.Now().Add(1*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user2")
		assert.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token3", tokens[0].Name)
	})

	t.Run("MarkTokensAsExpired", func(t *testing.T) {
		err := store.MarkTokensAsExpired(ctx, "test-ns", "user1")
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		assert.NoError(t, err)
		assert.Len(t, tokens, 2)

		for _, token := range tokens {
			assert.Equal(t, "expired", token.Status)
			assert.NotEmpty(t, token.ExpiredAt)
		}

		// Check status
		active, err := store.IsTokenActive(ctx, "hash1")
		assert.NoError(t, err)
		assert.False(t, active)

		// User2 should still be active
		tokens2, err := store.GetTokensForUser(ctx, "user2")
		assert.NoError(t, err)
		assert.Equal(t, "active", tokens2[0].Status)
	})

	t.Run("MarkSingleTokenAsExpired", func(t *testing.T) {
		// Create new token
		err := store.AddTokenMetadata(ctx, "test-ns", "user3", "token4", "hash4", time.Now().Add(1*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user3")
		assert.NoError(t, err)
		tokenID := tokens[0].ID

		err = store.MarkTokenAsExpired(ctx, tokenID, "user3")
		assert.NoError(t, err)

		active, err := store.IsTokenActive(ctx, "hash4")
		assert.NoError(t, err)
		assert.False(t, active)
	})
}
