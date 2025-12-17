package api_keys

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

var (
	ErrEmptyJTI  = errors.New("token JTI is required and cannot be empty")
	ErrEmptyName = errors.New("token name is required and cannot be empty")
)

type SQLStore struct {
	db     *sql.DB
	dbType DBType
}

var _ MetadataStore = (*SQLStore)(nil)

// NewSQLiteStore creates a SQLite store with a file path.
// Use ":memory:" for an in-memory database (ephemeral, for testing).
// Use a file path like "/data/maas-api.db" for persistent storage.
func NewSQLiteStore(ctx context.Context, dbPath string) (*SQLStore, error) {
	if dbPath == "" {
		dbPath = sqliteMemory
	}
	
	dsn := dbPath
	if dbPath != sqliteMemory {
		dsn = fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000", dbPath)
	}

	db, err := sql.Open(driverSQLite, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	configureConnectionPool(db, DBTypeSQLite)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	s := &SQLStore{db: db, dbType: DBTypeSQLite}
	if err := s.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if dbPath == sqliteMemory {
		log.Printf("Connected to SQLite in-memory database (ephemeral - data will be lost on restart)")
	} else {
		log.Printf("Connected to SQLite database at %s", dbPath)
	}
	return s, nil
}


func (s *SQLStore) Close() error {
	return s.db.Close()
}

func (s *SQLStore) initSchema(ctx context.Context) error {
	// Use TEXT for timestamps - works for both SQLite and PostgreSQL
	// SQLite doesn't have TIMESTAMPTZ, and TEXT is portable
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS tokens (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		creation_date TEXT NOT NULL,
		expiration_date TEXT NOT NULL
	)`

	if _, err := s.db.ExecContext(ctx, createTableQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tokens_username ON tokens(username)`); err != nil {
		return fmt.Errorf("failed to create username index: %w", err)
	}

	return nil
}

// placeholder returns the appropriate placeholder for the database type.
// SQLite uses ?, PostgreSQL uses $1, $2, etc.
func (s *SQLStore) placeholder(index int) string {
	return placeholder(s.dbType, index)
}

func (s *SQLStore) Add(ctx context.Context, username string, apiKey *APIKey) error {
	jti := strings.TrimSpace(apiKey.JTI)
	if jti == "" {
		return ErrEmptyJTI
	}

	name := strings.TrimSpace(apiKey.Name)
	if name == "" {
		return ErrEmptyName
	}

	var creationDate time.Time
	if apiKey.IssuedAt > 0 {
		creationDate = time.Unix(apiKey.IssuedAt, 0)
	} else {
		creationDate = time.Now()
	}
	expirationDate := time.Unix(apiKey.ExpiresAt, 0)

	// Store as RFC3339 strings for portability
	creationStr := creationDate.UTC().Format(time.RFC3339)
	expirationStr := expirationDate.UTC().Format(time.RFC3339)

	//nolint:gosec // G201: Safe - using placeholder indices, not user input
	query := fmt.Sprintf(`
	INSERT INTO tokens (id, username, name, description, creation_date, expiration_date)
	VALUES (%s, %s, %s, %s, %s, %s)
	`, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6))

	description := strings.TrimSpace(apiKey.Description)
	_, err := s.db.ExecContext(ctx, query, jti, username, name, description, creationStr, expirationStr)
	if err != nil {
		return fmt.Errorf("failed to insert token metadata: %w", err)
	}
	return nil
}

func (s *SQLStore) InvalidateAll(ctx context.Context, username string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	//nolint:gosec // G201: Safe - using placeholder indices, not user input
	query := fmt.Sprintf(`UPDATE tokens SET expiration_date = %s WHERE username = %s AND expiration_date > %s`,
		s.placeholder(1), s.placeholder(2), s.placeholder(3))

	result, err := s.db.ExecContext(ctx, query, now, username, now)
	if err != nil {
		return fmt.Errorf("failed to mark tokens as expired: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	log.Printf("Marked %d tokens as expired for user %s", rows, username)
	return nil
}

func (s *SQLStore) List(ctx context.Context, username string) ([]ApiKeyMetadata, error) {
	//nolint:gosec // G201: Safe - using placeholder indices, not user input
	query := fmt.Sprintf(`
	SELECT id, name, COALESCE(description, ''), creation_date, expiration_date
	FROM tokens 
	WHERE username = %s
	ORDER BY creation_date DESC
	`, s.placeholder(1))

	rows, err := s.db.QueryContext(ctx, query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	tokens := []ApiKeyMetadata{}

	for rows.Next() {
		var t ApiKeyMetadata
		var creationStr, expirationStr string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &creationStr, &expirationStr); err != nil {
			return nil, err
		}

		t.CreationDate = creationStr
		t.ExpirationDate = expirationStr
		t.Status = computeTokenStatus(expirationStr, now)

		tokens = append(tokens, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (s *SQLStore) Get(ctx context.Context, username, jti string) (*ApiKeyMetadata, error) {
	//nolint:gosec // G201: Safe - using placeholder indices, not user input
	query := fmt.Sprintf(`
	SELECT id, name, COALESCE(description, ''), creation_date, expiration_date
	FROM tokens 
	WHERE username = %s AND id = %s
	`, s.placeholder(1), s.placeholder(2))

	row := s.db.QueryRowContext(ctx, query, username, jti)

	var t ApiKeyMetadata
	var creationStr, expirationStr string
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &creationStr, &expirationStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	t.CreationDate = creationStr
	t.ExpirationDate = expirationStr
	t.Status = computeTokenStatus(expirationStr, time.Now())

	return &t, nil
}

func computeTokenStatus(expirationStr string, now time.Time) string {
	expirationDate, err := time.Parse(time.RFC3339, expirationStr)
	if err != nil || now.After(expirationDate) {
		return TokenStatusExpired
	}
	return TokenStatusActive
}
