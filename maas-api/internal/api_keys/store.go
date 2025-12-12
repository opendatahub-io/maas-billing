package api_keys

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ErrTokenNotFound is returned when a token is not found in the store.
var ErrTokenNotFound = errors.New("token not found")

const (
	// TokenStatusActive indicates the token is active.
	TokenStatusActive = "active"
	// TokenStatusExpired indicates the token has expired.
	TokenStatusExpired = "expired"
)

// Store handles the persistence of token metadata using SQLite.
type Store struct {
	db *sql.DB
}

// NewStore creates a new TokenStore backed by SQLite.
func NewStore(ctx context.Context, dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initSchema(ctx context.Context) error {
	// 1. Create table
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS tokens (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		namespace TEXT NOT NULL,
		creation_date TEXT NOT NULL,
		expiration_date TEXT NOT NULL
	);`
	if _, err := s.db.ExecContext(ctx, createTableQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// 2. Migrate existing tables: add description column if it doesn't exist
	// SQLite doesn't support "ALTER TABLE ... ADD COLUMN IF NOT EXISTS", so we check first
	var columnExists int
	checkColumnQuery := `
	SELECT COUNT(*) FROM pragma_table_info('tokens') WHERE name='description';
	`
	err := s.db.QueryRowContext(ctx, checkColumnQuery).Scan(&columnExists)
	if err == nil && columnExists == 0 {
		// Column doesn't exist, add it
		alterTableQuery := `ALTER TABLE tokens ADD COLUMN description TEXT;`
		if _, err := s.db.ExecContext(ctx, alterTableQuery); err != nil {
			return fmt.Errorf("failed to add description column: %w", err)
		}
		log.Printf("Added description column to tokens table")
	}

	// 3. Create indices
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tokens_username ON tokens(username)`); err != nil {
		return fmt.Errorf("failed to create username index: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tokens_username_namespace ON tokens(username, namespace)`); err != nil {
		return fmt.Errorf("failed to create username-namespace composite index: %w", err)
	}

	return nil
}

// AddTokenMetadata adds a new API key metadata to the database.
func (s *Store) AddTokenMetadata(ctx context.Context, namespace, username string, apiKey *APIKey) error {
	// Validate required fields
	jti := strings.TrimSpace(apiKey.JTI)
	if jti == "" {
		return errors.New("token JTI is required and cannot be empty")
	}

	name := strings.TrimSpace(apiKey.Name)
	if name == "" {
		return errors.New("token name is required and cannot be empty")
	}

	// Use JWT iat claim if available, otherwise fall back to current time
	var creationDate string
	if apiKey.IssuedAt > 0 {
		creationDate = time.Unix(apiKey.IssuedAt, 0).Format(time.RFC3339)
	} else {
		creationDate = time.Now().Format(time.RFC3339)
	}
	expirationDate := time.Unix(apiKey.ExpiresAt, 0).Format(time.RFC3339)

	query := `
	INSERT INTO tokens (id, username, name, description, namespace, creation_date, expiration_date)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	description := strings.TrimSpace(apiKey.Description)
	_, err := s.db.ExecContext(ctx, query, jti, username, name, description, namespace, creationDate, expirationDate)
	if err != nil {
		return fmt.Errorf("failed to insert token metadata: %w", err)
	}
	return nil
}

// MarkTokensAsExpiredForUser marks all active tokens for a user as expired by updating their expiration_date to the current time.
func (s *Store) MarkTokensAsExpiredForUser(ctx context.Context, namespace, username string) error {
	now := time.Now()
	expirationDate := now.Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339)

	// Update only tokens that are not already expired (expiration_date > now)
	// This ensures we only mark active tokens as expired, not ones already expired
	query := `UPDATE tokens SET expiration_date = ? WHERE username = ? AND namespace = ? AND expiration_date > ?`
	result, err := s.db.ExecContext(ctx, query, expirationDate, username, namespace, nowStr)
	if err != nil {
		return fmt.Errorf("failed to mark tokens as expired: %w", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("Marked %d tokens as expired for user %s", rows, username)
	return nil
}

// DeleteToken is deprecated and non-functional - kept for interface compatibility.
// Single token deletion is not supported in the initial release.
// Use MarkTokensAsExpiredForUser to revoke all tokens for a user.
func (s *Store) DeleteToken(ctx context.Context, namespace, username, jti string) error {
	// This method is intentionally non-functional - single token deletion removed for initial release
	return errors.New("single token deletion not supported - use DELETE /v1/tokens to revoke all tokens")
}

// GetTokensForUser retrieves all tokens for a user in a specific namespace.
func (s *Store) GetTokensForUser(ctx context.Context, namespace, username string) ([]ApiKeyMetadata, error) {
	query := `
	SELECT id, name, description, creation_date, expiration_date
	FROM tokens 
	WHERE username = ? AND namespace = ?
	ORDER BY creation_date DESC
	`
	rows, err := s.db.QueryContext(ctx, query, username, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	tokens := []ApiKeyMetadata{} // Initialize as empty slice to return [] instead of null

	for rows.Next() {
		var t ApiKeyMetadata
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreationDate, &t.ExpirationDate); err != nil {
			return nil, err
		}

		expiration, err := time.Parse(time.RFC3339, t.ExpirationDate)
		if err != nil {
			log.Printf("Failed to parse expiration date for token %s: %v", t.ID, err)
			t.Status = TokenStatusExpired // Mark as expired if date is unreadable
		} else {
			if now.After(expiration) {
				t.Status = TokenStatusExpired
			} else {
				t.Status = TokenStatusActive
			}
		}

		tokens = append(tokens, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tokens, nil
}

// GetToken retrieves a single token for a user by its JTI in a specific namespace.
func (s *Store) GetToken(ctx context.Context, namespace, username, jti string) (*ApiKeyMetadata, error) {
	query := `
	SELECT id, name, description, creation_date, expiration_date
	FROM tokens 
	WHERE username = ? AND namespace = ? AND id = ?
	`
	row := s.db.QueryRowContext(ctx, query, username, namespace, jti)

	var t ApiKeyMetadata
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &t.CreationDate, &t.ExpirationDate); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	expiration, err := time.Parse(time.RFC3339, t.ExpirationDate)
	if err != nil {
		log.Printf("Failed to parse expiration date for token %s: %v", t.ID, err)
		t.Status = TokenStatusExpired
	} else {
		if time.Now().After(expiration) {
			t.Status = TokenStatusExpired
		} else {
			t.Status = TokenStatusActive
		}
	}

	return &t, nil
}
