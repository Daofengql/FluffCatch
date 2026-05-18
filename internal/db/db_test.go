package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteMigrationsCreateCurrentSchema(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "fluffcatch.db")

	gormDB, err := OpenGORM(ctx, dsn, Options{Driver: DriverSQLite, Quiet: true})
	if err != nil {
		t.Fatalf("OpenGORM() returned error: %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("DB() returned error: %v", err)
	}
	defer sqlDB.Close()

	if err := Migrate(ctx, sqlDB, DriverSQLite); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	for _, table := range []string{"settings", "events", "photos", "submissions", "submission_links"} {
		var count int
		if err := sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	var applied int
	if err := sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", "001_initial_schema.sql").Scan(&applied); err != nil {
		t.Fatalf("check schema migration: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected sqlite initial migration to be recorded")
	}
}

func TestSQLiteNeedsInitialMigration(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.db")
	needs, err := SQLiteNeedsInitialMigration(missing)
	if err != nil {
		t.Fatalf("SQLiteNeedsInitialMigration(missing) returned error: %v", err)
	}
	if !needs {
		t.Fatal("expected missing sqlite file to need initial migration")
	}

	empty := filepath.Join(dir, "empty.db")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatalf("write empty sqlite file: %v", err)
	}
	needs, err = SQLiteNeedsInitialMigration(empty)
	if err != nil {
		t.Fatalf("SQLiteNeedsInitialMigration(empty) returned error: %v", err)
	}
	if !needs {
		t.Fatal("expected empty sqlite file to need initial migration")
	}

	existing := filepath.Join(dir, "existing.db")
	if err := os.WriteFile(existing, []byte("not empty"), 0600); err != nil {
		t.Fatalf("write existing sqlite file: %v", err)
	}
	needs, err = SQLiteNeedsInitialMigration(existing)
	if err != nil {
		t.Fatalf("SQLiteNeedsInitialMigration(existing) returned error: %v", err)
	}
	if needs {
		t.Fatal("expected existing non-empty sqlite file to skip initial migration")
	}
}
