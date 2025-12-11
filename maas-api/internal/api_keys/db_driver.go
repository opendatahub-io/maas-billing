package api_keys

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3"    // SQLite driver
	"k8s.io/utils/env"
)

type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"

	driverSQLite   = "sqlite3"
	driverPostgres = "pgx"

	sqliteMemory = ":memory:"
)

// parseConnectionString determines the database type and returns the appropriate driver and DSN.
// Returns an error if the connection string format is unrecognized.
func parseConnectionString(connStr string) (DBType, string, string, error) {
	connStr = strings.TrimSpace(connStr)

	if strings.HasPrefix(connStr, "postgresql://") || strings.HasPrefix(connStr, "postgres://") {
		return DBTypePostgres, driverPostgres, connStr, nil
	}

	if path, found := strings.CutPrefix(connStr, "sqlite://"); found {
		if path == "" || path == sqliteMemory {
			return DBTypeSQLite, driverSQLite, sqliteMemory, nil
		}
		return DBTypeSQLite, driverSQLite, path + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000", nil
	}

	if strings.HasPrefix(connStr, "file:") {
		return DBTypeSQLite, driverSQLite, connStr, nil
	}

	if connStr == sqliteMemory {
		return DBTypeSQLite, driverSQLite, sqliteMemory, nil
	}

	if strings.HasSuffix(connStr, ".db") || strings.HasSuffix(connStr, ".sqlite") || strings.HasSuffix(connStr, ".sqlite3") {
		return DBTypeSQLite, driverSQLite, connStr + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000", nil
	}

	return "", "", "", fmt.Errorf(
		"unrecognized database URL format: %q. Supported formats: postgresql://..., sqlite:///path, file:path, :memory:, or .db/.sqlite/.sqlite3 file paths",
		connStr)
}

// placeholder returns the appropriate SQL placeholder for the database type.
// SQLite uses ?, PostgreSQL uses $1, $2, etc.
func placeholder(dbType DBType, index int) string {
	if dbType == DBTypeSQLite {
		return "?"
	}
	return fmt.Sprintf("$%d", index)
}

const (
	defaultMaxOpenConns        = 25
	defaultMaxIdleConns        = 5
	defaultConnMaxLifetimeSecs = 300
)

// configureConnectionPool sets optimal connection pool settings based on database type.
func configureConnectionPool(db *sql.DB, dbType DBType) {
	if dbType == DBTypePostgres {
		maxOpenConns, _ := env.GetInt("DB_MAX_OPEN_CONNS", defaultMaxOpenConns)
		maxIdleConns, _ := env.GetInt("DB_MAX_IDLE_CONNS", defaultMaxIdleConns)
		connMaxLifetimeSecs, _ := env.GetInt("DB_CONN_MAX_LIFETIME_SECONDS", defaultConnMaxLifetimeSecs)

		db.SetMaxOpenConns(maxOpenConns)
		db.SetMaxIdleConns(maxIdleConns)
		db.SetConnMaxLifetime(time.Duration(connMaxLifetimeSecs) * time.Second)
	} else {
		// SQLite: single connection to avoid database locking issues
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)
	}
}

