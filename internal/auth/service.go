package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fluffcatch/internal/config"
	appdb "fluffcatch/internal/db"

	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"
)

const (
	passwordHashAlgorithm  = "pbkdf2-sha256"
	passwordHashIterations = 210000
	passwordSaltBytes      = 16
	generatedPasswordBytes = 18
)

type Service struct {
	db            *gorm.DB
	configManager *config.Manager
}

func NewService(dbConn *gorm.DB, configManager *config.Manager) *Service {
	return &Service{
		db:            dbConn,
		configManager: configManager,
	}
}

func (service *Service) Login(ctx context.Context, req LoginRequest) LoginResponse {
	cfg := service.currentConfig()
	if strings.TrimSpace(req.Username) != cfg.Auth.AdminUsername {
		return LoginResponse{
			Authenticated: false,
			Message:       "用户名或密码错误",
		}
	}
	if strings.TrimSpace(cfg.Auth.AdminPasswordHash) == "" {
		return LoginResponse{
			Authenticated: false,
			Message:       "管理员密码尚未初始化",
		}
	}

	ok, err := VerifyPassword(req.Password, cfg.Auth.AdminPasswordHash)
	if err != nil || !ok {
		return LoginResponse{
			Authenticated: false,
			Message:       "用户名或密码错误",
		}
	}

	return LoginResponse{
		Authenticated: true,
		Message:       "登录成功",
		Username:      cfg.Auth.AdminUsername,
	}
}

func (service *Service) CreateSession(ctx context.Context, username string, ttl time.Duration) (string, time.Time, error) {
	if service.db == nil {
		return "", time.Time{}, fmt.Errorf("database-backed sessions are not available")
	}
	cfg := service.currentConfig()
	if strings.TrimSpace(username) != cfg.Auth.AdminUsername {
		return "", time.Time{}, fmt.Errorf("admin user not found")
	}

	sessionID, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().Add(ttl)
	if err := service.db.WithContext(ctx).Create(&appdb.Session{ID: sessionID, Username: cfg.Auth.AdminUsername, ExpiresAt: expiresAt}).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}

	return sessionID, expiresAt, nil
}

func (service *Service) AuthenticateSession(ctx context.Context, sessionID string) (AdminUser, bool, error) {
	if service.db == nil || strings.TrimSpace(sessionID) == "" {
		return AdminUser{}, false, nil
	}

	var session appdb.Session
	err := service.db.WithContext(ctx).
		Where("id = ? AND expires_at > CURRENT_TIMESTAMP", sessionID).
		Take(&session).Error
	if err == gorm.ErrRecordNotFound {
		return AdminUser{}, false, nil
	}
	if err != nil {
		return AdminUser{}, false, fmt.Errorf("authenticate session: %w", err)
	}

	cfg := service.currentConfig()
	if session.Username != cfg.Auth.AdminUsername {
		return AdminUser{}, false, nil
	}

	return AdminUser{Username: cfg.Auth.AdminUsername}, true, nil
}

func (service *Service) Logout(ctx context.Context, sessionID string) error {
	if service.db == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}

	if err := service.db.WithContext(ctx).Where("id = ?", sessionID).Delete(&appdb.Session{}).Error; err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (service *Service) GetOIDCStatus(ctx context.Context, enabled bool, providerName string) (OIDCStatus, error) {
	_ = ctx
	cfg := service.currentConfig()
	return OIDCStatus{
		Enabled:      enabled,
		Bound:        strings.TrimSpace(cfg.OIDC.BoundSubject) != "",
		Subject:      cfg.OIDC.BoundSubject,
		ProviderName: providerName,
	}, nil
}

func (service *Service) OIDCAllowed(info OIDCIdentity) bool {
	cfg := service.currentConfig()
	return oidcIdentityAllowed(cfg.OIDC, info)
}

func (service *Service) BindOIDC(ctx context.Context, subject string) error {
	_ = ctx
	if service.configManager == nil {
		return fmt.Errorf("config manager is not available")
	}
	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("oidc subject is required")
	}
	_, err := service.configManager.BindOIDC(subject)
	return err
}

func (service *Service) UnbindOIDC(ctx context.Context) error {
	_ = ctx
	if service.configManager == nil {
		return fmt.Errorf("config manager is not available")
	}
	_, err := service.configManager.UnbindOIDC()
	return err
}

func (service *Service) ChangePassword(ctx context.Context, currentPassword string, newPassword string, currentSessionID string) error {
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("new password is required")
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	cfg := service.currentConfig()
	ok, err := VerifyPassword(currentPassword, cfg.Auth.AdminPasswordHash)
	if err != nil || !ok {
		return fmt.Errorf("current password is incorrect")
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if service.configManager == nil {
		return fmt.Errorf("config manager is not available")
	}
	if _, err := service.configManager.UpdateAdminPasswordHash(newHash); err != nil {
		return fmt.Errorf("update config password hash: %w", err)
	}

	if service.db != nil {
		query := service.db.WithContext(ctx)
		if strings.TrimSpace(currentSessionID) != "" {
			query = query.Where("id <> ?", currentSessionID)
		}
		if err := query.Delete(&appdb.Session{}).Error; err != nil {
			return fmt.Errorf("invalidate other sessions: %w", err)
		}
	}

	return nil
}

func EnsureConfigAdminPassword(ctx context.Context, manager *config.Manager) (string, bool, error) {
	_ = ctx
	if manager == nil {
		return "", false, fmt.Errorf("config manager is required")
	}
	cfg := manager.Current()
	if strings.TrimSpace(cfg.Auth.AdminPasswordHash) != "" {
		return "", false, nil
	}

	password, err := GeneratePassword()
	if err != nil {
		return "", false, err
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return "", false, err
	}
	if _, err := manager.UpdateAdminPasswordHash(passwordHash); err != nil {
		return "", false, err
	}
	return password, true, nil
}

func EnsureConfigSessionSecret(ctx context.Context, manager *config.Manager) (string, bool, error) {
	_ = ctx
	if manager == nil {
		return "", false, fmt.Errorf("config manager is required")
	}
	cfg := manager.Current()
	if strings.TrimSpace(cfg.Auth.SessionSecret) != "" && cfg.Auth.SessionSecret != "change-me-in-production" {
		return "", false, nil
	}

	secret, err := randomHex(32)
	if err != nil {
		return "", false, err
	}
	if _, err := manager.UpdateSessionSecret(secret); err != nil {
		return "", false, err
	}
	return secret, true, nil
}

func ResetConfigAdminPassword(ctx context.Context, manager *config.Manager, dbConn *gorm.DB, password string) (string, error) {
	_ = ctx
	if manager == nil {
		return "", fmt.Errorf("config manager is required")
	}
	if password == "" {
		generated, err := GeneratePassword()
		if err != nil {
			return "", err
		}
		password = generated
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return "", err
	}
	if _, err := manager.UpdateAdminPasswordHash(passwordHash); err != nil {
		return "", err
	}
	if dbConn != nil {
		if err := dbConn.WithContext(ctx).Where("1 = 1").Delete(&appdb.Session{}).Error; err != nil {
			return "", fmt.Errorf("invalidate sessions after password reset: %w", err)
		}
	}

	return password, nil
}

type OIDCIdentity struct {
	Subject string
}

func oidcIdentityAllowed(oidcConfig config.OIDCConfig, info OIDCIdentity) bool {
	if strings.TrimSpace(oidcConfig.BoundSubject) != "" {
		return strings.TrimSpace(info.Subject) == strings.TrimSpace(oidcConfig.BoundSubject)
	}
	return false
}

func (service *Service) currentConfig() config.Config {
	if service.configManager == nil {
		return config.Config{}
	}
	return service.configManager.Current()
}

func GeneratePassword() (string, error) {
	raw := make([]byte, generatedPasswordBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomHex(bytesLength int) (string, error) {
	raw := make([]byte, bytesLength)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}

	return hex.EncodeToString(raw), nil
}

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password is required")
	}

	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := pbkdf2.Key([]byte(password), salt, passwordHashIterations, sha256.Size, sha256.New)
	return fmt.Sprintf(
		"%s$%d$%s$%s",
		passwordHashAlgorithm,
		passwordHashIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func VerifyPassword(password string, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false, fmt.Errorf("invalid password hash format")
	}
	if parts[0] != passwordHashAlgorithm {
		return false, fmt.Errorf("unsupported password hash algorithm")
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, fmt.Errorf("invalid password hash iterations: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, fmt.Errorf("decode password salt: %w", err)
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}

	actual := pbkdf2.Key([]byte(password), salt, iterations, len(expected), sha256.New)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}
