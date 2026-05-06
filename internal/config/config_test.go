package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("STORAGE_DRIVER", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.App.Name != "FluffCatch" {
		t.Fatalf("expected default app name, got %q", cfg.App.Name)
	}

	if cfg.Storage.Driver != "local" {
		t.Fatalf("expected default storage driver local, got %q", cfg.Storage.Driver)
	}

	if cfg.Database.Host != "127.0.0.1" {
		t.Fatalf("expected default mysql host, got %q", cfg.Database.Host)
	}

	if cfg.Database.Port != 3306 {
		t.Fatalf("expected default mysql port, got %d", cfg.Database.Port)
	}

	if cfg.Database.MaxOpenConns != 20 {
		t.Fatalf("expected default mysql max open conns 20, got %d", cfg.Database.MaxOpenConns)
	}

	if cfg.Database.MaxIdleConns != 10 {
		t.Fatalf("expected default mysql max idle conns 10, got %d", cfg.Database.MaxIdleConns)
	}

	if cfg.Database.ConnMaxLifetime != 25*time.Minute {
		t.Fatalf("expected default mysql conn max lifetime 25m, got %s", cfg.Database.ConnMaxLifetime)
	}

	if cfg.Database.ConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("expected default mysql conn max idle time 5m, got %s", cfg.Database.ConnMaxIdleTime)
	}

	if cfg.Database.Timeout != 5*time.Second {
		t.Fatalf("expected default mysql timeout 5s, got %s", cfg.Database.Timeout)
	}

	if cfg.Database.ReadTimeout != 30*time.Second {
		t.Fatalf("expected default mysql read timeout 30s, got %s", cfg.Database.ReadTimeout)
	}

	if cfg.Database.WriteTimeout != 30*time.Second {
		t.Fatalf("expected default mysql write timeout 30s, got %s", cfg.Database.WriteTimeout)
	}

	if cfg.Database.ConnectRetries != 5 {
		t.Fatalf("expected default mysql connect retries 5, got %d", cfg.Database.ConnectRetries)
	}

	if cfg.Database.ConnectRetryDelay != 2*time.Second {
		t.Fatalf("expected default mysql connect retry delay 2s, got %s", cfg.Database.ConnectRetryDelay)
	}
}

func TestLoadRejectsUnknownStorageDriver(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "ftp")

	_, err := Load()
	if err == nil {
		t.Fatal("expected unsupported storage driver error")
	}
}

func TestLoadDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":8088")
	t.Setenv("STORAGE_DRIVER", "local")
	t.Chdir(t.TempDir())

	if err := os.WriteFile(".env", []byte("HTTP_ADDR=\":9090\"\nAPP_ENV=testing\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.HTTP.Addr != ":8088" {
		t.Fatalf("expected environment to win, got %q", cfg.HTTP.Addr)
	}

	if cfg.App.Env != "testing" {
		t.Fatalf("expected APP_ENV from .env, got %q", cfg.App.Env)
	}
}

func TestDatabaseDSNFromSeparateFields(t *testing.T) {
	cfg := DatabaseConfig{
		Host:         "db.local",
		Port:         3307,
		User:         "fluff",
		Password:     "secret",
		Database:     "catch",
		Charset:      "utf8mb4",
		Location:     "Asia/Shanghai",
		ParseTime:    true,
		Timeout:      3 * time.Second,
		ReadTimeout:  11 * time.Second,
		WriteTimeout: 13 * time.Second,
	}

	dsn, err := cfg.DSN()
	if err != nil {
		t.Fatalf("DSN() returned error: %v", err)
	}

	expectedParts := []string{
		"fluff:secret@tcp(db.local:3307)/catch?",
		"charset=utf8mb4",
		"parseTime=true",
		"loc=Asia%2FShanghai",
		"timeout=3s",
		"readTimeout=11s",
		"writeTimeout=13s",
	}

	for _, expected := range expectedParts {
		if !strings.Contains(dsn, expected) {
			t.Fatalf("expected DSN %q to contain %q", dsn, expected)
		}
	}
}
