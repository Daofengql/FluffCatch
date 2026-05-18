package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fluffcatch/internal/config"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("fluffy-secret")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}

	ok, err := VerifyPassword("fluffy-secret", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = VerifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() returned error for wrong password: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}

func TestLoginUsesConfigPasswordHash(t *testing.T) {
	hash, err := HashPassword("fluffy-secret")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	cfg, err := config.LoadFile("")
	if err != nil {
		t.Fatalf("LoadFile() returned error: %v", err)
	}
	cfg.Auth.AdminPasswordHash = hash

	service := NewService(config.NewManager("", cfg))

	result := service.Login(context.Background(), LoginRequest{Username: "admin", Password: "fluffy-secret"})
	if !result.Authenticated || result.Username != "admin" {
		t.Fatalf("expected config-backed login to succeed, got %#v", result)
	}

	result = service.Login(context.Background(), LoginRequest{Username: "admin", Password: "wrong"})
	if result.Authenticated {
		t.Fatal("expected wrong password to fail")
	}
}

func TestChangePasswordUpdatesConfigManager(t *testing.T) {
	hash, err := HashPassword("old-secret")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	cfg, err := config.LoadFile("")
	if err != nil {
		t.Fatalf("LoadFile() returned error: %v", err)
	}
	cfg.Auth.AdminPasswordHash = hash
	manager := config.NewManager("", cfg)
	service := NewService(manager)

	if err := service.ChangePassword(context.Background(), "old-secret", "new-secret", ""); err != nil {
		t.Fatalf("ChangePassword() returned error: %v", err)
	}
	ok, err := VerifyPassword("new-secret", manager.Current().Auth.AdminPasswordHash)
	if err != nil {
		t.Fatalf("VerifyPassword() returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected updated config password hash to verify")
	}
}

func TestEnsureConfigSessionSecretWritesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
auth:
  admin_username: admin
  admin_password_hash: pbkdf2-sha256$210000$salt$hash
`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() returned error: %v", err)
	}
	manager := config.NewManager(path, cfg)

	secret, generated, err := EnsureConfigSessionSecret(context.Background(), manager)
	if err != nil {
		t.Fatalf("EnsureConfigSessionSecret() returned error: %v", err)
	}
	if !generated || len(secret) < 32 {
		t.Fatalf("expected generated session secret, got generated=%v secret=%q", generated, secret)
	}
	if manager.Current().Auth.SessionSecret != secret {
		t.Fatal("expected current config to include generated session secret")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(raw), "session_secret: "+secret) {
		t.Fatalf("expected session_secret to be written to config:\n%s", string(raw))
	}

	nextSecret, nextGenerated, err := EnsureConfigSessionSecret(context.Background(), manager)
	if err != nil {
		t.Fatalf("EnsureConfigSessionSecret() second call returned error: %v", err)
	}
	if nextGenerated || nextSecret != "" {
		t.Fatalf("expected existing session secret to be preserved, got generated=%v secret=%q", nextGenerated, nextSecret)
	}
}
