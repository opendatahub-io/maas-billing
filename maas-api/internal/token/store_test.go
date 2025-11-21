package token

import (
	"context"
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

	ctx := context.Background()

	t.Run("AddTokenMetadata", func(t *testing.T) {
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", "token1", "jti1", time.Now().Add(1*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		assert.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token1", tokens[0].Name)
		assert.Equal(t, "active", tokens[0].Status)
	})

	t.Run("AddSecondToken", func(t *testing.T) {
		err := store.AddTokenMetadata(ctx, "test-ns", "user1", "token2", "jti2", time.Now().Add(2*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		assert.NoError(t, err)
		assert.Len(t, tokens, 2)
	})

	t.Run("GetTokensForDifferentUser", func(t *testing.T) {
		err := store.AddTokenMetadata(ctx, "test-ns", "user2", "token3", "jti3", time.Now().Add(1*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user2")
		assert.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "token3", tokens[0].Name)
	})

	t.Run("DeleteTokensForUser", func(t *testing.T) {
		err := store.DeleteTokensForUser(ctx, "test-ns", "user1")
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user1")
		assert.NoError(t, err)
		assert.Len(t, tokens, 0)

		// User2 should still exist
		tokens2, err := store.GetTokensForUser(ctx, "user2")
		assert.NoError(t, err)
		assert.Len(t, tokens2, 1)
	})

	t.Run("GetToken", func(t *testing.T) {
		// Retrieve user2's token by JTI
		token, err := store.GetToken(ctx, "user2", "jti3")
		assert.NoError(t, err)
		assert.NotNil(t, token)
		assert.Equal(t, "token3", token.Name)
	})

	t.Run("ExpiredTokenStatus", func(t *testing.T) {
		// Add an expired token
		err := store.AddTokenMetadata(ctx, "test-ns", "user4", "expired-token", "jti-expired", time.Now().Add(-1*time.Hour).Unix())
		assert.NoError(t, err)

		tokens, err := store.GetTokensForUser(ctx, "user4")
		assert.NoError(t, err)
		assert.Len(t, tokens, 1)
		assert.Equal(t, "expired", tokens[0].Status)

		// Get single token check
		token, err := store.GetToken(ctx, "user4", "jti-expired")
		assert.NoError(t, err)
		assert.Equal(t, "expired", token.Status)
	})
}
