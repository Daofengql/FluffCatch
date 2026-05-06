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
		Name     string `json:"name"`
		Subtitle string `json:"subtitle"`
		LogoURL  string `json:"logoUrl"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode site response: %v", err)
	}

	if payload.Name != "FluffCatch" {
		t.Fatalf("expected fallback site name, got %q", payload.Name)
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
