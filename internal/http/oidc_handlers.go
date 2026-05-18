package http

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	stdhttp "net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/config"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type oidcFlowState struct {
	Action      string
	Username    string
	RedirectURL string
	ReturnBase  string
	Nonce       string
	ExpiresAt   time.Time
}

type oidcStateStore struct {
	mu      sync.Mutex
	entries map[string]oidcFlowState
}

type oidcUserInfo struct {
	Subject string
}

func newOIDCStateStore() *oidcStateStore {
	return &oidcStateStore{entries: map[string]oidcFlowState{}}
}

func (store *oidcStateStore) Put(token string, state oidcFlowState) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.entries[token] = state
	store.pruneLocked()
}

func (store *oidcStateStore) Consume(token string) (oidcFlowState, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	state, ok := store.entries[token]
	if ok {
		delete(store.entries, token)
	}
	store.pruneLocked()
	if !ok || time.Now().After(state.ExpiresAt) {
		return oidcFlowState{}, false
	}
	return state, true
}

func (store *oidcStateStore) pruneLocked() {
	now := time.Now()
	for token, state := range store.entries {
		if now.After(state.ExpiresAt) {
			delete(store.entries, token)
		}
	}
}

func (server *Server) oidcLoginURL(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	authURL, err := server.startOIDCFlow(r, "login", "")
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"url": authURL})
}

func (server *Server) oidcBindURL(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	admin, ok, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "admin authentication required")
		return
	}
	authURL, err := server.startOIDCFlow(r, "bind", admin.Username)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"url": authURL})
}

func (server *Server) oidcCallback(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	stateToken := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || stateToken == "" {
		server.redirectOIDCResult(w, r, oidcFlowState{}, "缺少 Keycloak 回调参数")
		return
	}
	flow, ok := server.oidcStates.Consume(stateToken)
	if !ok {
		server.redirectOIDCResult(w, r, oidcFlowState{}, "Keycloak 登录状态已过期，请重试")
		return
	}

	oidcSettings, err := server.currentOIDCSettings(r.Context())
	if err != nil || !oidcSettings.Enabled {
		server.redirectOIDCResult(w, r, flow, "OIDC 未启用")
		return
	}
	redirectURL := flow.RedirectURL
	if redirectURL == "" {
		redirectURL = server.oidcRedirectURL(r)
	}
	oauthConfig, verifier, err := oidcRuntime(r.Context(), oidcSettings, redirectURL)
	if err != nil {
		server.redirectOIDCResult(w, r, flow, "OIDC 配置不可用")
		return
	}
	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		server.redirectOIDCResult(w, r, flow, "Keycloak 授权码交换失败")
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		server.redirectOIDCResult(w, r, flow, "Keycloak 未返回 id_token")
		return
	}
	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		server.redirectOIDCResult(w, r, flow, "Keycloak id_token 校验失败")
		return
	}
	if flow.Nonce == "" || idToken.Nonce != flow.Nonce {
		server.redirectOIDCResult(w, r, flow, "Keycloak nonce 校验失败")
		return
	}
	info := oidcUserInfo{Subject: idToken.Subject}

	switch flow.Action {
	case "bind":
		if err := server.authService.BindOIDC(r.Context(), info.Subject); err != nil {
			server.redirectOIDCResult(w, r, flow, err.Error())
			return
		}
		server.redirectOIDCResult(w, r, flow, "")
	default:
		if !server.authService.OIDCAllowed(auth.OIDCIdentity{Subject: info.Subject}) {
			flow.Action = "login"
			server.redirectOIDCResult(w, r, flow, "该 OIDC 账号未被允许登录后台")
			return
		}
		sessionID, expiresAt, err := server.authService.CreateSession(r.Context(), server.configManager.Current().Auth.AdminUsername, 30*24*time.Hour)
		if err != nil {
			flow.Action = "login"
			server.redirectOIDCResult(w, r, flow, "创建登录会话失败")
			return
		}
		setSessionCookie(w, r, server.cfg.App.Env, sessionID, expiresAt)
		flow.Action = "login"
		server.redirectOIDCResult(w, r, flow, "")
	}
}

func (server *Server) oidcStatus(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	_, ok, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "admin authentication required")
		return
	}
	oidcSettings, _ := server.currentOIDCSettings(r.Context())
	status, err := server.authService.GetOIDCStatus(r.Context(), oidcSettings.Enabled, oidcProviderName(oidcSettings))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusOK, status)
}

func (server *Server) oidcUnbind(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	_, ok, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "admin authentication required")
		return
	}
	if err := server.authService.UnbindOIDC(r.Context()); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"message": "oidc account unbound"})
}

func (server *Server) publicOIDCSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	oidcSettings, err := server.currentOIDCSettings(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load oidc settings")
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"enabled":      oidcSettings.Enabled,
		"providerName": oidcProviderName(oidcSettings),
	})
}

func (server *Server) startOIDCFlow(r *stdhttp.Request, action string, username string) (string, error) {
	oidcSettings, err := server.currentOIDCSettings(r.Context())
	if err != nil {
		return "", fmt.Errorf("failed to load oidc settings")
	}
	if !oidcSettings.Enabled {
		return "", fmt.Errorf("OIDC 未启用")
	}
	redirectURL := server.oidcRedirectURL(r)
	oauthConfig, _, err := oidcRuntime(r.Context(), oidcSettings, redirectURL)
	if err != nil {
		return "", fmt.Errorf("OIDC 配置不可用")
	}
	state, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("failed to create oidc state")
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("failed to create oidc nonce")
	}
	server.oidcStates.Put(state, oidcFlowState{
		Action:      action,
		Username:    username,
		RedirectURL: redirectURL,
		ReturnBase:  server.oidcReturnBaseURL(r),
		Nonce:       nonce,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	})
	return oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce)), nil
}

func (server *Server) redirectOIDCResult(w stdhttp.ResponseWriter, r *stdhttp.Request, flow oidcFlowState, message string) {
	targetPath := "/login"
	if flow.Action == "bind" {
		targetPath = "/admin/settings/security"
	}
	u := url.URL{Path: targetPath}
	if flow.ReturnBase != "" {
		if parsed, err := url.Parse(flow.ReturnBase); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			u.Scheme = parsed.Scheme
			u.Host = parsed.Host
		}
	}
	query := u.Query()
	if message != "" {
		query.Set("oidc_error", message)
	} else if flow.Action == "bind" {
		query.Set("oidc_success", "Keycloak 绑定成功")
	} else {
		query.Set("oidc_success", "Keycloak 登录成功")
	}
	u.RawQuery = query.Encode()
	stdhttp.Redirect(w, r, u.String(), stdhttp.StatusFound)
}

func oidcRuntime(ctx context.Context, oidcSettings config.OIDCConfig, redirectURL string) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	issuer := strings.TrimRight(strings.TrimSpace(oidcSettings.IssuerURL), "/")
	redirectURL = strings.TrimSpace(redirectURL)
	if issuer == "" || strings.TrimSpace(oidcSettings.ClientID) == "" || redirectURL == "" {
		return nil, nil, fmt.Errorf("OIDC issuerUrl, clientId and callback URL are required")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, nil, err
	}
	oauthConfig := &oauth2.Config{
		ClientID:     strings.TrimSpace(oidcSettings.ClientID),
		ClientSecret: strings.TrimSpace(oidcSettings.ClientSecret),
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       []string{oidc.ScopeOpenID},
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: oauthConfig.ClientID})
	return oauthConfig, verifier, nil
}

func (server *Server) oidcRedirectURL(r *stdhttp.Request) string {
	scheme := "http"
	if r.TLS != nil || server.cfg.App.Env == "production" {
		scheme = "https"
	}
	if forwardedProto := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}

	host := r.Host
	if forwardedHost := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	if host == "" {
		host = "localhost"
	}

	prefix := strings.TrimRight(strings.TrimSpace(firstForwardedValue(r.Header.Get("X-Forwarded-Prefix"))), "/")
	return fmt.Sprintf("%s://%s%s/api/v1/auth/oidc/callback", scheme, host, prefix)
}

func (server *Server) oidcReturnBaseURL(r *stdhttp.Request) string {
	for _, raw := range []string{
		r.Header.Get("Origin"),
		r.Header.Get("Referer"),
		server.requestBaseURL(r),
	} {
		if parsed, err := url.Parse(strings.TrimSpace(raw)); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
		}
	}
	return ""
}

func (server *Server) requestBaseURL(r *stdhttp.Request) string {
	scheme := "http"
	if r.TLS != nil || server.cfg.App.Env == "production" {
		scheme = "https"
	}
	if forwardedProto := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	host := r.Host
	if forwardedHost := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	if host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func firstForwardedValue(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}

func (server *Server) currentOIDCSettings(ctx context.Context) (config.OIDCConfig, error) {
	_ = ctx
	return server.configManager.Current().OIDC, nil
}

func oidcProviderName(oidcSettings config.OIDCConfig) string {
	name := strings.TrimSpace(oidcSettings.Provider)
	if name == "" {
		return "Keycloak"
	}
	return name
}

func randomURLToken(length int) (string, error) {
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
