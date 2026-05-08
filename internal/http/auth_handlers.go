package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"fluffcatch/internal/auth"
)

func (server *Server) captcha(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.captchaLimiter.Allow(clientIP(r.RemoteAddr)) {
		writeError(w, stdhttp.StatusTooManyRequests, "too many captcha requests, please slow down")
		return
	}
	challenge, err := server.captchaStore.NewChallenge(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to create captcha")
		return
	}

	writeJSON(w, stdhttp.StatusOK, challenge)
}

func (server *Server) login(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.loginLimiter.Allow(clientIP(r.RemoteAddr)) {
		writeError(w, stdhttp.StatusTooManyRequests, "too many login attempts, please slow down")
		return
	}
	var req auth.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid login payload")
		return
	}

	if !server.captchaStore.Verify(r.Context(), req.CaptchaID, req.CaptchaAnswer) {
		writeError(w, stdhttp.StatusBadRequest, "invalid captcha")
		return
	}

	result := server.authService.Login(r.Context(), req)
	if !result.Authenticated {
		writeJSON(w, stdhttp.StatusUnauthorized, result)
		return
	}

	sessionID, expiresAt, err := server.authService.CreateSession(r.Context(), req.Username, 30*24*time.Hour)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to create session")
		return
	}

	setSessionCookie(w, r, server.cfg.App.Env, sessionID, expiresAt)

	writeJSON(w, stdhttp.StatusOK, result)
}

func setSessionCookie(w stdhttp.ResponseWriter, r *stdhttp.Request, env string, sessionID string, expiresAt time.Time) {
	secure := r.TLS != nil || env == "production"
	stdhttp.SetCookie(w, &stdhttp.Cookie{
		Name:     "fluffcatch_session",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   secure,
	})
}

func (server *Server) logout(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if cookie, err := r.Cookie("fluffcatch_session"); err == nil {
		_ = server.authService.Logout(r.Context(), cookie.Value)
	}
	secure := r.TLS != nil || server.cfg.App.Env == "production"
	stdhttp.SetCookie(w, &stdhttp.Cookie{
		Name:     "fluffcatch_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   secure,
	})
	writeJSON(w, stdhttp.StatusOK, map[string]string{
		"message": "logged out",
	})
}

func (server *Server) me(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	user, ok, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}
	if !ok {
		writeJSON(w, stdhttp.StatusOK, auth.MeResponse{Authenticated: false})
		return
	}

	writeJSON(w, stdhttp.StatusOK, auth.MeResponse{Authenticated: true, Username: user.Username})
}

func (server *Server) currentAdmin(r *stdhttp.Request) (auth.AdminUser, bool, error) {
	cookie, err := r.Cookie("fluffcatch_session")
	if err != nil {
		return auth.AdminUser{}, false, nil
	}
	return server.authService.AuthenticateSession(r.Context(), cookie.Value)
}

const eventPrivateAccessTokenTTL = 24 * time.Hour

func (server *Server) eventPrivateAccessUnlocked(r *stdhttp.Request, eventID int64) bool {
	cookie, err := r.Cookie(eventPrivateAccessCookieName(eventID))
	if err != nil {
		return false
	}
	return server.verifyEventPrivateAccessToken(cookie.Value, eventID)
}

func eventPrivateAccessCookieName(eventID int64) string {
	return fmt.Sprintf("fluffcatch_private_%d", eventID)
}

func (server *Server) signEventPrivateAccessToken(eventID int64) string {
	issuedAt := strconv.FormatInt(time.Now().Unix(), 10)
	payload := strings.Join([]string{"v1", strconv.FormatInt(eventID, 10), issuedAt}, ".")
	signature := server.signEventPrivateAccessPayload(payload)
	return payload + "." + signature
}

func (server *Server) verifyEventPrivateAccessToken(token string, eventID int64) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != strconv.FormatInt(eventID, 10) {
		return false
	}
	issuedUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return false
	}
	issuedAt := time.Unix(issuedUnix, 0)
	if time.Since(issuedAt) > eventPrivateAccessTokenTTL || issuedAt.After(time.Now().Add(5*time.Minute)) {
		return false
	}

	payload := strings.Join(parts[:3], ".")
	expected := server.signEventPrivateAccessPayload(payload)
	return hmac.Equal([]byte(expected), []byte(parts[3]))
}

func (server *Server) signEventPrivateAccessPayload(payload string) string {
	secret := server.cfg.Auth.SessionSecret
	if secret == "" {
		secret = "fluffcatch-private-access-development"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
