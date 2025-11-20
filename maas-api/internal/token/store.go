package token

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// NamedToken represents metadata for a single token
type NamedToken struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CreationDate   string `json:"creationDate"`
	ExpirationDate string `json:"expirationDate"`
	Status         string `json:"status"` // "active", "expired"
	ExpiredAt      string `json:"expiredAt,omitempty"`
}

// Store handles the persistence of token metadata using SQLite
type Store struct {
	db *sql.DB
}

// NewStore creates a new TokenStore backed by SQLite
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS tokens (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		name TEXT NOT NULL,
		namespace TEXT,
		creation_date TEXT NOT NULL,
		expiration_date TEXT NOT NULL,
		status TEXT DEFAULT 'active',
		expired_at TEXT,
		token_hash TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_tokens_username ON tokens(username);
	CREATE INDEX IF NOT EXISTS idx_tokens_hash ON tokens(token_hash);
	`
	_, err := s.db.Exec(query)
	if err != nil {
		// Try adding column if table exists but column doesn't (migration)
		if err.Error() != "" {
			log.Printf("Attempting migration for token_hash: %v", err)
			_, _ = s.db.Exec("ALTER TABLE tokens ADD COLUMN token_hash TEXT")
			_, _ = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_tokens_hash ON tokens(token_hash)")
		}
	}
	return nil
}

// AddTokenMetadata adds a new token to the database
func (s *Store) AddTokenMetadata(ctx context.Context, namespace, username, tokenName, tokenHash string, expiresAt int64) error {
	now := time.Now()
	tokenID := s.generateTokenID(username, tokenName, now)
	creationDate := now.Format(time.RFC3339)
	expirationDate := time.Unix(expiresAt, 0).Format(time.RFC3339)

	query := `
	INSERT INTO tokens (id, username, name, namespace, creation_date, expiration_date, status, token_hash)
	VALUES (?, ?, ?, ?, ?, ?, 'active', ?)
	`
	_, err := s.db.ExecContext(ctx, query, tokenID, username, tokenName, namespace, creationDate, expirationDate, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to insert token metadata: %w", err)
	}
	return nil
}

// MarkTokensAsExpired marks all active tokens for a user as expired
func (s *Store) MarkTokensAsExpired(ctx context.Context, namespace, username string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
	UPDATE tokens 
	SET status = 'expired', expired_at = ? 
	WHERE username = ? AND status = 'active'
	`
	result, err := s.db.ExecContext(ctx, query, now, username)
	if err != nil {
		return fmt.Errorf("failed to expire tokens: %w", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("Expired %d tokens for user %s", rows, username)
	return nil
}

// MarkTokenAsExpired marks a single token as expired by ID
func (s *Store) MarkTokenAsExpired(ctx context.Context, tokenID, username string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
	UPDATE tokens 
	SET status = 'expired', expired_at = ? 
	WHERE id = ? AND username = ? AND status = 'active'
	`
	result, err := s.db.ExecContext(ctx, query, now, tokenID, username)
	if err != nil {
		return fmt.Errorf("failed to expire token %s: %w", tokenID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found or already expired")
	}
	return nil
}

// GetTokensForUser retrieves all tokens for a user
func (s *Store) GetTokensForUser(ctx context.Context, username string) ([]NamedToken, error) {
	query := `
	SELECT id, name, creation_date, expiration_date, status, expired_at 
	FROM tokens 
	WHERE username = ?
	ORDER BY creation_date DESC
	`
	rows, err := s.db.QueryContext(ctx, query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []NamedToken
	for rows.Next() {
		var t NamedToken
		var expiredAt sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.CreationDate, &t.ExpirationDate, &t.Status, &expiredAt); err != nil {
			return nil, err
		}
		if expiredAt.Valid {
			t.ExpiredAt = expiredAt.String
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

// IsTokenActive checks if a token with the given hash is active
func (s *Store) IsTokenActive(ctx context.Context, tokenHash string) (bool, error) {
	query := `SELECT status FROM tokens WHERE token_hash = ?`
	var status string
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			// Token not found in metadata (might be an old token or non-tracked one)
			// Decide policy: Allow untracked?
			// For now, we return true (allow) if it's not explicitly revoked in DB.
			// However, the "Deny List" logic implies we only block if it IS in DB and is EXPIRED.
			return true, nil
		}
		return false, err
	}
	return status == "active", nil
}

func (s *Store) generateTokenID(username, name string, t time.Time) string {
	data := fmt.Sprintf("%s-%s-%d", username, name, t.UnixNano())
	sum := sha1.Sum([]byte(data))
	return hex.EncodeToString(sum[:])[:12]
}
