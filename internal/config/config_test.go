package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Chdir(t.TempDir())

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

	if cfg.Upload.MaxSizeMB != 20 {
		t.Fatalf("expected default upload max size 20, got %d", cfg.Upload.MaxSizeMB)
	}

	if cfg.Upload.MaxVideoSizeMB != 500 {
		t.Fatalf("expected default upload max video size 500, got %d", cfg.Upload.MaxVideoSizeMB)
	}

	if cfg.Upload.MaxFilesPerUpload != 20 {
		t.Fatalf("expected default upload max files 20, got %d", cfg.Upload.MaxFilesPerUpload)
	}
}

func TestLoadRejectsUnknownStorageDriver(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "ftp")
	t.Chdir(t.TempDir())

	_, err := Load()
	if err == nil {
		t.Fatal("expected unsupported storage driver error")
	}
}

func TestLoadFileReadsSpecifiedYAMLFile(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.WriteFile("config.production.yaml", []byte(`
app:
  env: production
http:
  addr: :8092
  read_timeout: 11s
database:
  user: fluffcatch
  password: ""
  database: fluffcatch
  conn_max_lifetime: 26m
auth:
  session_secret: secret
`), 0644); err != nil {
		t.Fatalf("write config.production.yaml: %v", err)
	}

	cfg, err := LoadFile("config.production.yaml")
	if err != nil {
		t.Fatalf("LoadFile() returned error: %v", err)
	}

	if cfg.HTTP.Addr != ":8092" {
		t.Fatalf("expected HTTP addr from YAML, got %q", cfg.HTTP.Addr)
	}

	if cfg.App.Env != "production" {
		t.Fatalf("expected app env from YAML, got %q", cfg.App.Env)
	}

	if cfg.Database.Password != "" {
		t.Fatalf("expected blank mysql password from YAML, got %q", cfg.Database.Password)
	}

	if cfg.HTTP.ReadTimeout != 11*time.Second {
		t.Fatalf("expected HTTP read timeout from YAML, got %s", cfg.HTTP.ReadTimeout)
	}

	if cfg.Database.ConnMaxLifetime != 26*time.Minute {
		t.Fatalf("expected mysql conn max lifetime from YAML, got %s", cfg.Database.ConnMaxLifetime)
	}
}

func TestEnvironmentOverridesYAML(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("MYSQL_USER", "from_env")
	t.Chdir(t.TempDir())

	if err := os.WriteFile("config.yaml", []byte(`
http:
  addr: :8092
database:
  user: from_yaml
`), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.HTTP.Addr != ":9999" {
		t.Fatalf("expected environment HTTP_ADDR to win, got %q", cfg.HTTP.Addr)
	}

	if cfg.Database.User != "from_env" {
		t.Fatalf("expected environment MYSQL_USER to win, got %q", cfg.Database.User)
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
