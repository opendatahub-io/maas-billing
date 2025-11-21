package token

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
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
		if strings.Contains(err.Error(), "duplicate column name") {
			// Column already exists, ignore
		} else {
			log.Printf("Schema init failed/incomplete, attempting migration for token_hash: %v", err)
			// Attempt migration anyway (e.g. if table existed but column didn't)
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
// Automatically marks tokens as expired if they've passed their expiration date
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

	now := time.Now()
	var tokens []NamedToken
	var tokensToExpire []string // Track tokens that need to be marked as expired

	for rows.Next() {
		var t NamedToken
		var expiredAt sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.CreationDate, &t.ExpirationDate, &t.Status, &expiredAt); err != nil {
			return nil, err
		}
		if expiredAt.Valid {
			t.ExpiredAt = expiredAt.String
		}

		// Check if token has expired based on expiration_date
		if t.Status == "active" {
			expirationDate, err := time.Parse(time.RFC3339, t.ExpirationDate)
			if err == nil && now.After(expirationDate) {
				// Token has expired, mark it
				t.Status = "expired"
				expiredAtTime := now.Format(time.RFC3339)
				t.ExpiredAt = expiredAtTime
				tokensToExpire = append(tokensToExpire, t.ID)
			}
		}

		tokens = append(tokens, t)
	}

	// Batch update expired tokens
	if len(tokensToExpire) > 0 {
		expiredAtTime := now.Format(time.RFC3339)
		placeholders := make([]string, len(tokensToExpire))
		args := make([]interface{}, len(tokensToExpire)+1)
		args[0] = expiredAtTime
		for i, id := range tokensToExpire {
			placeholders[i] = "?"
			args[i+1] = id
		}
		query := fmt.Sprintf(`UPDATE tokens SET status = 'expired', expired_at = ? WHERE id IN (%s) AND status = 'active'`, 
			strings.Join(placeholders, ","))
		_, _ = s.db.ExecContext(ctx, query, args...)
	}

	return tokens, nil
}

// IsTokenActive checks if a token with the given hash is active
// It checks both the status field and whether the token has passed its expiration date
func (s *Store) IsTokenActive(ctx context.Context, tokenHash string) (bool, error) {
	query := `SELECT status, expiration_date FROM tokens WHERE token_hash = ?`
	var status, expirationDateStr string
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(&status, &expirationDateStr)
	if err != nil {
		if err == sql.ErrNoRows {
			// Token not found in metadata.
			// Policy: Allow-list behavior.
			// Tokens must be explicitly present and marked "active" in the DB to be accepted.
			return false, nil
		}
		return false, err
	}

	// Check if status is active
	if status != "active" {
		return false, nil
	}

	// Check if token has expired based on expiration_date
	expirationDate, err := time.Parse(time.RFC3339, expirationDateStr)
	if err != nil {
		log.Printf("Failed to parse expiration_date %s: %v", expirationDateStr, err)
		// If we can't parse the date, consider it expired for safety
		return false, nil
	}

	// If expiration date has passed, token is no longer active
	if time.Now().After(expirationDate) {
		// Automatically mark as expired
		now := time.Now().Format(time.RFC3339)
		_, _ = s.db.ExecContext(ctx, `UPDATE tokens SET status = 'expired', expired_at = ? WHERE token_hash = ? AND status = 'active'`, now, tokenHash)
		return false, nil
	}

	return true, nil
}

// MarkExpiredTokens marks all tokens that have passed their expiration_date as expired
// This is used by background cleanup jobs
func (s *Store) MarkExpiredTokens(ctx context.Context) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	query := `
	UPDATE tokens 
	SET status = 'expired', expired_at = ?
	WHERE status = 'active' 
	AND expiration_date < ?
	AND (expired_at IS NULL OR expired_at = '')
	`
	result, err := s.db.ExecContext(ctx, query, now, now)
	if err != nil {
		return 0, fmt.Errorf("failed to mark expired tokens: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Marked %d expired tokens as expired", rowsAffected)
	}
	return rowsAffected, nil
}

func (s *Store) generateTokenID(username, name string, t time.Time) string {
	data := fmt.Sprintf("%s-%s-%d", username, name, t.UnixNano())
	sum := sha1.Sum([]byte(data))
	return hex.EncodeToString(sum[:])[:12]
}
