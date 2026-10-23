package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Diagnose returns actionable information without exposing the DSN or server messages.
func Diagnose(err error) string {
	var serverErr *pgconn.PgError
	if errors.As(err, &serverErr) {
		switch serverErr.Code {
		case "28P01":
			return "authentication failed: check PG_USER and PG_PASSWORD"
		case "28000":
			return "access denied: check role and pg_hba.conf"
		case "3D000":
			return "database does not exist: create PG_DATABASE before starting"
		case "53300":
			return "too many connections on PostgreSQL"
		default:
			return "PostgreSQL rejected the connection (SQLSTATE " + serverErr.Code + ")"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "connection timed out: check PG_HOST, PG_PORT and firewall"
	}
	var networkErr *net.OpError
	if errors.As(err, &networkErr) {
		return "network connection failed: check PG_HOST, PG_PORT and PostgreSQL listener"
	}
	return "connection failed: check PG_* configuration, SSL mode and PostgreSQL logs"
}

// DiagnoseMigration describes schema failures without logging SQL or server messages.
func DiagnoseMigration(err error) string {
	var fileErr *os.PathError
	if errors.As(err, &fileErr) {
		return "cannot read initial migration: include migrations/001_initial.up.sql in the deployment and start from the application root"
	}
	var serverErr *pgconn.PgError
	if errors.As(err, &serverErr) {
		var reason string
		switch serverErr.Code {
		case "42501":
			reason = "insufficient privileges: migration role needs USAGE and CREATE on the target schema and ownership of affected objects"
		case "42710", "42P07", "42723":
			reason = "schema object already exists: inspect existing schema and search_path before applying the consolidated migration; do not delete existing data"
		case "42601":
			reason = "SQL syntax error: check the deployed migration and PostgreSQL version"
		case "42804":
			reason = "incompatible existing schema type: compare expediente_init_type_enum with the initial migration before upgrading; existing data was not deleted"
		case "3F000":
			reason = "target schema is missing or search_path has no valid schema"
		default:
			reason = "migration rejected: inspect PostgreSQL server logs for the failing statement"
		}
		return "SQLSTATE " + serverErr.Code + ": " + reason
	}
	return Diagnose(err)
}

func Open(ctx context.Context, dsn string) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, nil, err
	}
	pool, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)
	if err = pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, nil, err
	}
	return db, pool, nil
}

// ApplyInitialMigration installs the consolidated schema once on a new database.
// Existing databases are preserved and are not upgraded by this bootstrap.
func ApplyInitialMigration(ctx context.Context, db *gorm.DB, migrationPath string) (bool, error) {
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT to_regclass('public.expedientes') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	sql, err := os.ReadFile(filepath.Clean(migrationPath))
	if err != nil {
		return false, fmt.Errorf("read initial migration: %w", err)
	}
	if err := db.WithContext(ctx).Exec(string(sql)).Error; err != nil {
		return false, err
	}
	return true, nil
}
