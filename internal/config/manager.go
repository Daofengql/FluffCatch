package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Manager struct {
	mu      sync.RWMutex
	path    string
	current Config
}

func NewManager(path string, cfg Config) *Manager {
	return &Manager{
		path:    strings.TrimSpace(path),
		current: cfg,
	}
}

func (manager *Manager) Current() Config {
	if manager == nil {
		return defaultConfig()
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.current
}

func (manager *Manager) UpdateAdminPasswordHash(passwordHash string) (Config, error) {
	passwordHash = strings.TrimSpace(passwordHash)
	if passwordHash == "" {
		return Config{}, fmt.Errorf("admin password hash is required")
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	next := manager.current
	next.Auth.AdminPasswordHash = passwordHash
	normalized, err := normalizeAndValidate(next)
	if err != nil {
		return Config{}, err
	}
	if err := patchYAMLFile(manager.path, map[string]map[string]any{
		"auth": {
			"admin_username":      normalized.Auth.AdminUsername,
			"admin_password_hash": normalized.Auth.AdminPasswordHash,
		},
	}); err != nil {
		return Config{}, err
	}

	manager.current = normalized
	return normalized, nil
}

func (manager *Manager) UpdateSessionSecret(sessionSecret string) (Config, error) {
	sessionSecret = strings.TrimSpace(sessionSecret)
	if sessionSecret == "" {
		return Config{}, fmt.Errorf("session secret is required")
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	next := manager.current
	next.Auth.SessionSecret = sessionSecret
	normalized, err := normalizeAndValidate(next)
	if err != nil {
		return Config{}, err
	}
	if err := patchYAMLFile(manager.path, map[string]map[string]any{
		"auth": {
			"admin_username":      normalized.Auth.AdminUsername,
			"admin_password_hash": normalized.Auth.AdminPasswordHash,
			"session_secret":      normalized.Auth.SessionSecret,
		},
	}); err != nil {
		return Config{}, err
	}

	manager.current = normalized
	return normalized, nil
}

func (manager *Manager) BindOIDC(subject string) (Config, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	next := manager.current
	next.OIDC.BoundSubject = strings.TrimSpace(subject)
	normalized, err := normalizeAndValidate(next)
	if err != nil {
		return Config{}, err
	}
	if err := patchYAMLFile(manager.path, map[string]map[string]any{
		"oidc": oidcPatch(normalized.OIDC),
	}); err != nil {
		return Config{}, err
	}

	manager.current = normalized
	return normalized, nil
}

func (manager *Manager) UnbindOIDC() (Config, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	next := manager.current
	next.OIDC.BoundSubject = ""
	normalized, err := normalizeAndValidate(next)
	if err != nil {
		return Config{}, err
	}
	if err := patchYAMLFile(manager.path, map[string]map[string]any{
		"oidc": oidcPatch(normalized.OIDC),
	}); err != nil {
		return Config{}, err
	}

	manager.current = normalized
	return normalized, nil
}

func oidcPatch(oidc OIDCConfig) map[string]any {
	return map[string]any{
		"enabled":       oidc.Enabled,
		"provider":      oidc.Provider,
		"issuer_url":    oidc.IssuerURL,
		"client_id":     oidc.ClientID,
		"client_secret": oidc.ClientSecret,
		"bound_subject": oidc.BoundSubject,
	}
}

func patchYAMLFile(path string, patches map[string]map[string]any) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	content := map[string]any{}
	raw, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		if err := yaml.Unmarshal(raw, &content); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	for section, values := range patches {
		current, _ := content[section].(map[string]any)
		if current == nil {
			current = map[string]any{}
		}
		for key, value := range values {
			current[key] = value
		}
		content[section] = current
	}

	encoded, err := yaml.Marshal(content)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
	}

	tempDir := dir
	if tempDir == "" {
		tempDir = "."
	}
	tempFile, err := os.CreateTemp(tempDir, ".fluffcatch-config-*")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tempName := tempFile.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	if _, err := tempFile.Write(encoded); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp config file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp config file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
