package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fluffcatch/migrations"

	glebarezsqlite "github.com/glebarez/sqlite"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	DriverMySQL  = "mysql"
	DriverSQLite = "sqlite"
)

type Options struct {
	Driver            string
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   time.Duration
	ConnMaxIdleTime   time.Duration
	ConnectRetries    int
	ConnectRetryDelay time.Duration
	Quiet             bool
}

func Open(ctx context.Context, dsn string, options Options) (*sql.DB, error) {
	driver, err := normalizeDriver(options.Driver)
	if err != nil {
		return nil, err
	}
	if driver == DriverSQLite {
		gormDB, err := openSQLiteGORM(ctx, dsn, options, &gorm.Config{})
		if err != nil {
			return nil, err
		}
		sqlDB, err := gormDB.DB()
		if err != nil {
			return nil, fmt.Errorf("access sqlite handle: %w", err)
		}
		return sqlDB, nil
	}

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	options = normalizeOptions(driver, options)
	conn.SetMaxOpenConns(options.MaxOpenConns)
	conn.SetMaxIdleConns(options.MaxIdleConns)
	conn.SetConnMaxLifetime(options.ConnMaxLifetime)
	conn.SetConnMaxIdleTime(options.ConnMaxIdleTime)

	if err := pingWithRetry(ctx, conn, options, driver); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return conn, nil
}

func OpenGORM(ctx context.Context, dsn string, options Options) (*gorm.DB, error) {
	gormConfig := &gorm.Config{}
	if options.Quiet {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	driver, err := normalizeDriver(options.Driver)
	if err != nil {
		return nil, err
	}
	if driver == DriverSQLite {
		return openSQLiteGORM(ctx, dsn, options, gormConfig)
	}

	sqlDB, err := Open(ctx, dsn, options)
	if err != nil {
		return nil, err
	}
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), gormConfig)
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("open gorm mysql: %w", err)
	}

	return gormDB.WithContext(ctx), nil
}

func openSQLiteGORM(ctx context.Context, dsn string, options Options, gormConfig *gorm.Config) (*gorm.DB, error) {
	if err := prepareSQLiteFile(dsn); err != nil {
		return nil, err
	}
	gormDB, err := gorm.Open(glebarezsqlite.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open gorm sqlite: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("access sqlite handle: %w", err)
	}

	options = normalizeOptions(DriverSQLite, options)
	sqlDB.SetMaxOpenConns(options.MaxOpenConns)
	sqlDB.SetMaxIdleConns(options.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(options.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(options.ConnMaxIdleTime)
	if err := applySQLitePragmas(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := pingWithRetry(ctx, sqlDB, options, DriverSQLite); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return gormDB.WithContext(ctx), nil
}

func normalizeDriver(driver string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", DriverMySQL:
		return DriverMySQL, nil
	case DriverSQLite:
		return DriverSQLite, nil
	default:
		return "", fmt.Errorf("unsupported database driver %q", driver)
	}
}

func normalizeOptions(driver string, options Options) Options {
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
	if driver == DriverSQLite {
		options.MaxOpenConns = 1
		if options.MaxIdleConns > 1 {
			options.MaxIdleConns = 1
		}
		options.ConnMaxLifetime = 0
		options.ConnMaxIdleTime = 0
	}

	return options
}

func prepareSQLiteFile(dsn string) error {
	path, ok := sqlitePersistentPath(dsn)
	if !ok {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create sqlite directory: %w", err)
		}
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat sqlite database: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("sqlite database path is a directory: %s", path)
	}
	if info.Size() == 0 {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove empty sqlite file: %w", err)
		}
	}
	return nil
}

func SQLiteNeedsInitialMigration(dsn string) (bool, error) {
	path, ok := sqlitePersistentPath(dsn)
	if !ok {
		return true, nil
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat sqlite database: %w", err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("sqlite database path is a directory: %s", path)
	}
	return info.Size() == 0, nil
}

func sqlitePersistentPath(dsn string) (string, bool) {
	value := strings.TrimSpace(dsn)
	if value == "" || value == ":memory:" {
		return "", false
	}
	if strings.HasPrefix(value, "file:") {
		if strings.HasPrefix(value, "file::memory:") {
			return "", false
		}
		if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
			path, _ := url.PathUnescape(parsed.Path)
			if parsed.Host != "" {
				path = "//" + parsed.Host + path
			}
			return filepath.FromSlash(path), true
		}
		value = strings.TrimPrefix(value, "file:")
	}
	if index := strings.Index(value, "?"); index >= 0 {
		value = value[:index]
	}
	if unescaped, err := url.PathUnescape(value); err == nil {
		value = unescaped
	}
	value = strings.TrimSpace(value)
	if value == "" || value == ":memory:" {
		return "", false
	}
	return filepath.Clean(value), true
}

func applySQLitePragmas(ctx context.Context, conn *sql.DB) error {
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply sqlite pragma %q: %w", statement, err)
		}
	}
	return nil
}

func pingWithRetry(ctx context.Context, conn *sql.DB, options Options, driver string) error {
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
			"database ping failed; retrying",
			"driver",
			driver,
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

func Migrate(ctx context.Context, conn *sql.DB, driver string) error {
	driver, err := normalizeDriver(driver)
	if err != nil {
		return err
	}
	if err := createSchemaMigrationsTable(ctx, conn, driver); err != nil {
		return err
	}

	migrationDir := "."
	if driver == DriverSQLite {
		migrationDir = "sqlite"
	}
	entries, err := migrations.FS.ReadDir(migrationDir)
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
		version := filepath.Base(name)
		applied, err := migrationApplied(ctx, conn, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		migrationPath := name
		if migrationDir != "." {
			migrationPath = filepath.ToSlash(filepath.Join(migrationDir, name))
		}
		content, err := migrations.FS.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}

		if err := applyMigration(ctx, conn, version, string(content)); err != nil {
			return err
		}
	}

	return nil
}

func createSchemaMigrationsTable(ctx context.Context, conn *sql.DB, driver string) error {
	statement := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version varchar(191) NOT NULL,
			applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (version)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`
	if driver == DriverSQLite {
		statement = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version text NOT NULL PRIMARY KEY,
				applied_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`
	}
	if _, err := conn.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
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
