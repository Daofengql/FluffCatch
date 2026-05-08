package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"fluffcatch/migrations"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Options struct {
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   time.Duration
	ConnMaxIdleTime   time.Duration
	ConnectRetries    int
	ConnectRetryDelay time.Duration
	Quiet             bool
}

func Open(ctx context.Context, dsn string, options Options) (*sql.DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	options = normalizeOptions(options)
	conn.SetMaxOpenConns(options.MaxOpenConns)
	conn.SetMaxIdleConns(options.MaxIdleConns)
	conn.SetConnMaxLifetime(options.ConnMaxLifetime)
	conn.SetConnMaxIdleTime(options.ConnMaxIdleTime)

	if err := pingWithRetry(ctx, conn, options); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return conn, nil
}

func OpenGORM(ctx context.Context, dsn string, options Options) (*gorm.DB, error) {
	sqlDB, err := Open(ctx, dsn, options)
	if err != nil {
		return nil, err
	}

	gormConfig := &gorm.Config{}
	if options.Quiet {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), gormConfig)
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("open gorm mysql: %w", err)
	}

	return gormDB.WithContext(ctx), nil
}

func normalizeOptions(options Options) Options {
	if options.MaxOpenConns <= 0 {
		options.MaxOpenConns = 20
	}
	if options.MaxIdleConns <= 0 {
		options.MaxIdleConns = 10
	}
	if options.MaxIdleConns > options.MaxOpenConns {
		options.MaxIdleConns = options.MaxOpenConns
	}
	if options.ConnMaxLifetime <= 0 {
		options.ConnMaxLifetime = 25 * time.Minute
	}
	if options.ConnMaxIdleTime <= 0 {
		options.ConnMaxIdleTime = 5 * time.Minute
	}
	if options.ConnectRetries < 0 {
		options.ConnectRetries = 0
	}
	if options.ConnectRetryDelay <= 0 {
		options.ConnectRetryDelay = 2 * time.Second
	}

	return options
}

func pingWithRetry(ctx context.Context, conn *sql.DB, options Options) error {
	attempts := options.ConnectRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := conn.PingContext(ctx); err != nil {
			lastErr = err
		} else {
			return nil
		}

		if attempt == attempts {
			break
		}

		slog.Warn(
			"mysql ping failed; retrying",
			"attempt",
			attempt,
			"max_attempts",
			attempts,
			"delay",
			options.ConnectRetryDelay,
			"error",
			lastErr,
		)

		timer := time.NewTimer(options.ConnectRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}

	return lastErr
}

func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version varchar(191) NOT NULL,
			applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (version)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := migrationApplied(ctx, conn, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if err := applyMigration(ctx, conn, name, string(content)); err != nil {
			return err
		}
	}

	return nil
}

func migrationApplied(ctx context.Context, conn *sql.DB, version string) (bool, error) {
	var exists int
	err := conn.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ? LIMIT 1", version).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}

	return false, fmt.Errorf("check migration %s: %w", version, err)
}

func applyMigration(ctx context.Context, conn *sql.DB, version string, statement string) error {
	for _, sqlStatement := range splitSQLStatements(statement) {
		if _, err := conn.ExecContext(ctx, sqlStatement); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
	}

	if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}

	return nil
}

func splitSQLStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
	}

	return statements
}
