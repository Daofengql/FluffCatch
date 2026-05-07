package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fluffcatch/internal/config"
	"fluffcatch/internal/settings"
	"fluffcatch/internal/storage"
)

func TestHealthRoute(t *testing.T) {
	handler := testServer(t).Routes()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload["status"] != "ok" {
		t.Fatalf("expected ok status, got %#v", payload["status"])
	}
}

func TestAdminRouteRequiresPlaceholderHeader(t *testing.T) {
	handler := testServer(t).Routes()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/submissions", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestLoginRejectsInvalidCaptcha(t *testing.T) {
	handler := testServer(t).Routes()
	captchaReq := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/captcha", nil)
	captchaRec := httptest.NewRecorder()
	handler.ServeHTTP(captchaRec, captchaReq)
	if captchaRec.Code != stdhttp.StatusOK {
		t.Fatalf("expected captcha status 200, got %d", captchaRec.Code)
	}

	var captchaPayload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(captchaRec.Body).Decode(&captchaPayload); err != nil {
		t.Fatalf("decode captcha: %v", err)
	}

	body := strings.NewReader(`{"username":"admin","password":"change-me","captchaId":"` + captchaPayload.ID + `","captchaAnswer":"0000"}`)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestPublicSiteRouteReturnsFallbackSettings(t *testing.T) {
	handler := testServer(t).Routes()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/site", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload struct {
		Name              string `json:"name"`
		HomeMarkdown      string `json:"homeMarkdown"`
		ThemeMode         string `json:"themeMode"`
		ThemePreset       string `json:"themePreset"`
		ThemePrimaryColor string `json:"themePrimaryColor"`
		ICPNumber         string `json:"icpNumber"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode site response: %v", err)
	}

	if payload.Name != "FluffCatch" {
		t.Fatalf("expected fallback site name, got %q", payload.Name)
	}
	if payload.HomeMarkdown != "" {
		t.Fatalf("expected empty fallback home markdown, got %q", payload.HomeMarkdown)
	}
	if payload.ICPNumber != "" {
		t.Fatalf("expected empty fallback ICP number, got %q", payload.ICPNumber)
	}
	if payload.ThemeMode != "system" || payload.ThemePreset != "blue" || payload.ThemePrimaryColor != "#2563eb" {
		t.Fatalf("unexpected fallback theme settings: %#v", payload)
	}
}

func TestEventPrivateAccessTokenIsSignedAndScoped(t *testing.T) {
	server := testServer(t)
	server.cfg.Auth.SessionSecret = "test-secret"

	token := server.signEventPrivateAccessToken(42)
	if !server.verifyEventPrivateAccessToken(token, 42) {
		t.Fatal("expected signed private access token to verify")
	}
	if server.verifyEventPrivateAccessToken(token, 43) {
		t.Fatal("expected private access token to be scoped to one event")
	}

	tampered := token[:len(token)-1] + "x"
	if server.verifyEventPrivateAccessToken(tampered, 42) {
		t.Fatal("expected tampered private access token to be rejected")
	}
}

func testServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.Config{
		App: config.AppConfig{
			Name: "FluffCatch",
			Env:  "test",
		},
		Storage: config.StorageConfig{
			Driver:       "local",
			LocalPath:    t.TempDir(),
			PublicPrefix: "/media",
		},
		Auth: config.AuthConfig{
			AdminUsername: "admin",
		},
		Upload: config.UploadConfig{
			MaxSizeMB:         20,
			MaxFilesPerUpload: 20,
		},
		Frontend: config.FrontendConfig{
			Mode:       "disabled",
			StaticRoot: t.TempDir(),
		},
	}

	storageManager, err := storage.NewManager("default-local", []storage.Config{
		{
			PolicyID:     "default-local",
			Name:         "默认本地存储",
			Driver:       "local",
			LocalPath:    cfg.Storage.LocalPath,
			PublicPrefix: cfg.Storage.PublicPrefix,
		},
	})
	if err != nil {
		t.Fatalf("create storage manager: %v", err)
	}

	settingsService := settings.NewService(settings.NewStore(nil, settings.FromConfig(cfg)))
	return NewServer(cfg, nil, storageManager, settingsService)
}
