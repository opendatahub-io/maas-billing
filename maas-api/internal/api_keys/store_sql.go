package api_keys

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type SQLStore struct {
	db *sql.DB
}

// Compile-time check that SQLStore implements Store interface.
var _ Store = (*SQLStore)(nil)

func NewSQLStore(ctx context.Context, databaseURL string) (*SQLStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &SQLStore{db: db}
	if err := s.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Printf("Connected to SQL database")
	return s, nil
}

// Closes the database connection.
func (s *SQLStore) Close() error {
	return s.db.Close()
}

func (s *SQLStore) initSchema(ctx context.Context) error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS tokens (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		namespace TEXT NOT NULL,
		creation_date TIMESTAMPTZ NOT NULL,
		expiration_date TIMESTAMPTZ NOT NULL
	);`
	if _, err := s.db.ExecContext(ctx, createTableQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tokens_username ON tokens(username)`); err != nil {
		return fmt.Errorf("failed to create username index: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tokens_username_namespace ON tokens(username, namespace)`); err != nil {
		return fmt.Errorf("failed to create username-namespace composite index: %w", err)
	}

	return nil
}

func (s *SQLStore) AddTokenMetadata(ctx context.Context, namespace, username string, apiKey *APIKey) error {
	jti := strings.TrimSpace(apiKey.JTI)
	if jti == "" {
		return fmt.Errorf("token JTI is required and cannot be empty")
	}

	name := strings.TrimSpace(apiKey.Name)
	if name == "" {
		return fmt.Errorf("token name is required and cannot be empty")
	}

	var creationDate time.Time
	if apiKey.IssuedAt > 0 {
		creationDate = time.Unix(apiKey.IssuedAt, 0)
	} else {
		creationDate = time.Now()
	}
	expirationDate := time.Unix(apiKey.ExpiresAt, 0)

	query := `
	INSERT INTO tokens (id, username, name, description, namespace, creation_date, expiration_date)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	description := strings.TrimSpace(apiKey.Description)
	_, err := s.db.ExecContext(ctx, query, jti, username, name, description, namespace, creationDate, expirationDate)
	if err != nil {
		return fmt.Errorf("failed to insert token metadata: %w", err)
	}
	return nil
}

func (s *SQLStore) MarkTokensAsExpiredForUser(ctx context.Context, namespace, username string) error {
	now := time.Now()

	query := `UPDATE tokens SET expiration_date = $1 WHERE username = $2 AND namespace = $3 AND expiration_date > $4`
	result, err := s.db.ExecContext(ctx, query, now, username, namespace, now)
	if err != nil {
		return fmt.Errorf("failed to mark tokens as expired: %w", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("Marked %d tokens as expired for user %s", rows, username)
	return nil
}

func (s *SQLStore) GetTokensForUser(ctx context.Context, namespace, username string) ([]ApiKeyMetadata, error) {
	query := `
	SELECT id, name, COALESCE(description, ''), creation_date, expiration_date
	FROM tokens 
	WHERE username = $1 AND namespace = $2
	ORDER BY creation_date DESC
	`
	rows, err := s.db.QueryContext(ctx, query, username, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	tokens := []ApiKeyMetadata{} 

	for rows.Next() {
		var t ApiKeyMetadata
		var creationDate, expirationDate time.Time
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &creationDate, &expirationDate); err != nil {
			return nil, err
		}

		t.CreationDate = creationDate.Format(time.RFC3339)
		t.ExpirationDate = expirationDate.Format(time.RFC3339)

		if now.After(expirationDate) {
			t.Status = TokenStatusExpired
		} else {
			t.Status = TokenStatusActive
		}

		tokens = append(tokens, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (s *SQLStore) GetToken(ctx context.Context, namespace, username, jti string) (*ApiKeyMetadata, error) {
	query := `
	SELECT id, name, COALESCE(description, ''), creation_date, expiration_date
	FROM tokens 
	WHERE username = $1 AND namespace = $2 AND id = $3
	`
	row := s.db.QueryRowContext(ctx, query, username, namespace, jti)

	var t ApiKeyMetadata
	var creationDate, expirationDate time.Time
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &creationDate, &expirationDate); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	t.CreationDate = creationDate.Format(time.RFC3339)
	t.ExpirationDate = expirationDate.Format(time.RFC3339)

	if time.Now().After(expirationDate) {
		t.Status = TokenStatusExpired
	} else {
		t.Status = TokenStatusActive
	}

	return &t, nil
}
