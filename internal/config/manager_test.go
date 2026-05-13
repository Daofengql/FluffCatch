package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerWritesAdminPasswordHashToConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
auth:
  admin_username: root
  session_secret: test-secret
`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() returned error: %v", err)
	}
	manager := NewManager(path, cfg)

	const passwordHash = "pbkdf2-sha256$210000$salt$hash"
	next, err := manager.UpdateAdminPasswordHash(passwordHash)
	if err != nil {
		t.Fatalf("UpdateAdminPasswordHash() returned error: %v", err)
	}
	if next.Auth.AdminPasswordHash != passwordHash {
		t.Fatalf("expected current config password hash to update")
	}

	reloaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Auth.AdminUsername != "root" || reloaded.Auth.AdminPasswordHash != passwordHash {
		t.Fatalf("expected written auth config, got %#v", reloaded.Auth)
	}
}

func TestManagerWritesAndClearsOIDCBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
auth:
  session_secret: test-secret
oidc:
  enabled: true
  provider: Keycloak
  issuer_url: https://kc.example.test/realms/fluffcatch
  client_id: fluffcatch
  client_secret: old-secret
`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() returned error: %v", err)
	}
	manager := NewManager(path, cfg)
	if _, err := manager.BindOIDC("subject-1"); err != nil {
		t.Fatalf("BindOIDC() returned error: %v", err)
	}

	reloaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.OIDC.ClientID != "fluffcatch" || reloaded.OIDC.BoundSubject != "subject-1" {
		t.Fatalf("expected OIDC binding in config file, got %#v", reloaded.OIDC)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	contents := string(raw)
	for _, disallowed := range []string{"redirect_url", "allowed_subject", "allowed_email", "allowed_username", "bound_email", "bound_username"} {
		if strings.Contains(contents, disallowed) {
			t.Fatalf("expected %s to be absent from config file:\n%s", disallowed, contents)
		}
	}

	unbound, err := manager.UnbindOIDC()
	if err != nil {
		t.Fatalf("UnbindOIDC() returned error: %v", err)
	}
	if unbound.OIDC.BoundSubject != "" {
		t.Fatalf("expected OIDC binding to be cleared, got %#v", unbound.OIDC)
	}
}
