package api_keys

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/opendatahub-io/maas-billing/maas-api/internal/token"
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
		namespace TEXT,
		creation_date TEXT NOT NULL,
		expiration_date TEXT NOT NULL
	);`
	if _, err := s.db.ExecContext(ctx, createTableQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// 2. Create indices
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tokens_username ON tokens(username)`); err != nil {
		return fmt.Errorf("failed to create username index: %w", err)
	}

	return nil
}

// AddTokenMetadata adds a new token to the database.
func (s *Store) AddTokenMetadata(ctx context.Context, namespace, username string, tok *token.Token) error {
	now := time.Now()
	creationDate := now.Format(time.RFC3339)
	expirationDate := time.Unix(tok.ExpiresAt, 0).Format(time.RFC3339)

	query := `
	INSERT INTO tokens (id, username, name, namespace, creation_date, expiration_date)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query, tok.JTI, username, tok.Name, namespace, creationDate, expirationDate)
	if err != nil {
		return fmt.Errorf("failed to insert token metadata: %w", err)
	}
	return nil
}

// DeleteTokensForUser deletes all tokens for a user from the database.
func (s *Store) DeleteTokensForUser(ctx context.Context, namespace, username string) error {
	query := `DELETE FROM tokens WHERE username = ? AND namespace = ?`
	result, err := s.db.ExecContext(ctx, query, username, namespace)
	if err != nil {
		return fmt.Errorf("failed to delete tokens: %w", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("Deleted %d tokens for user %s", rows, username)
	return nil
}

// DeleteToken deletes a single token for a user in a specific namespace.
func (s *Store) DeleteToken(ctx context.Context, namespace, username, jti string) error {
	query := `DELETE FROM tokens WHERE username = ? AND namespace = ? AND id = ?`
	result, err := s.db.ExecContext(ctx, query, username, namespace, jti)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// GetTokensForUser retrieves all tokens for a user in a specific namespace.
func (s *Store) GetTokensForUser(ctx context.Context, namespace, username string) ([]NamedToken, error) {
	query := `
	SELECT id, name, creation_date, expiration_date
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
	tokens := []NamedToken{} // Initialize as empty slice to return [] instead of null

	for rows.Next() {
		var t NamedToken
		if err := rows.Scan(&t.ID, &t.Name, &t.CreationDate, &t.ExpirationDate); err != nil {
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
func (s *Store) GetToken(ctx context.Context, namespace, username, jti string) (*NamedToken, error) {
	query := `
	SELECT id, name, creation_date, expiration_date
	FROM tokens 
	WHERE username = ? AND namespace = ? AND id = ?
	`
	row := s.db.QueryRowContext(ctx, query, username, namespace, jti)

	var t NamedToken
	if err := row.Scan(&t.ID, &t.Name, &t.CreationDate, &t.ExpirationDate); err != nil {
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

