package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	appdb "fluffcatch/internal/db"

	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	passwordHashAlgorithm  = "pbkdf2-sha256"
	passwordHashIterations = 210000
	passwordSaltBytes      = 16
	generatedPasswordBytes = 18
)

type Service struct {
	db               *gorm.DB
	fallbackUsername string
}

func NewService(dbConn *gorm.DB, fallbackUsername string) *Service {
	return &Service{
		db:               dbConn,
		fallbackUsername: fallbackUsername,
	}
}

func (service *Service) Login(ctx context.Context, req LoginRequest) LoginResponse {
	if service.db == nil {
		return LoginResponse{
			Authenticated: false,
			Message:       "database-backed admin login is not available",
		}
	}

	var passwordHash string
	err := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).
		Select("password_hash").
		Where("username = ?", req.Username).
		Take(&passwordHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LoginResponse{
			Authenticated: false,
			Message:       "invalid username or password",
		}
	}
	if err != nil {
		return LoginResponse{
			Authenticated: false,
			Message:       "failed to load admin user",
		}
	}

	ok, err := VerifyPassword(req.Password, passwordHash)
	if err != nil || !ok {
		return LoginResponse{
			Authenticated: false,
			Message:       "invalid username or password",
		}
	}

	return LoginResponse{
		Authenticated: true,
		Message:       "login accepted",
		Username:      req.Username,
	}
}

func (service *Service) CreateSession(ctx context.Context, username string, ttl time.Duration) (string, time.Time, error) {
	if service.db == nil {
		return "", time.Time{}, fmt.Errorf("database-backed sessions are not available")
	}

	var userID int64
	if err := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).Select("id").Where("username = ?", username).Take(&userID).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("load admin user for session: %w", err)
	}

	sessionID, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().Add(ttl)
	if err := service.db.WithContext(ctx).Create(&appdb.Session{ID: sessionID, AdminUserID: userID, ExpiresAt: expiresAt}).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}

	return sessionID, expiresAt, nil
}

func (service *Service) AuthenticateSession(ctx context.Context, sessionID string) (AdminUser, bool, error) {
	if service.db == nil || strings.TrimSpace(sessionID) == "" {
		return AdminUser{}, false, nil
	}

	var user AdminUser
	err := service.db.WithContext(ctx).
		Table("sessions").
		Select("admin_users.id, admin_users.username, admin_users.password_hash, admin_users.oidc_subject, admin_users.oidc_username, admin_users.oidc_email, admin_users.oidc_bound_at, admin_users.created_at, admin_users.updated_at").
		Joins("INNER JOIN admin_users ON admin_users.id = sessions.admin_user_id").
		Where("sessions.id = ? AND sessions.expires_at > CURRENT_TIMESTAMP", sessionID).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminUser{}, false, nil
	}
	if err != nil {
		return AdminUser{}, false, fmt.Errorf("authenticate session: %w", err)
	}

	return user, true, nil
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

func (service *Service) CreateSessionForUserID(ctx context.Context, userID int64, ttl time.Duration) (string, time.Time, error) {
	if service.db == nil {
		return "", time.Time{}, fmt.Errorf("database-backed sessions are not available")
	}
	sessionID, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(ttl)
	if err := service.db.WithContext(ctx).Create(&appdb.Session{ID: sessionID, AdminUserID: userID, ExpiresAt: expiresAt}).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return sessionID, expiresAt, nil
}

func (service *Service) GetOIDCStatus(ctx context.Context, username string, enabled bool, providerName string) (OIDCStatus, error) {
	status := OIDCStatus{Enabled: enabled, ProviderName: providerName}
	if service.db == nil {
		return status, nil
	}
	var user AdminUser
	err := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).
		Select("oidc_subject", "oidc_username", "oidc_email", "oidc_bound_at").
		Where("username = ?", username).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status, fmt.Errorf("admin user not found")
	}
	if err != nil {
		return status, fmt.Errorf("load oidc status: %w", err)
	}
	status.Bound = strings.TrimSpace(user.OIDCSubject) != ""
	status.Subject = user.OIDCSubject
	status.Username = user.OIDCUsername
	status.Email = user.OIDCEmail
	status.BoundAt = user.OIDCBoundAt
	return status, nil
}

func (service *Service) UserByOIDCSubject(ctx context.Context, subject string) (AdminUser, bool, error) {
	if service.db == nil || strings.TrimSpace(subject) == "" {
		return AdminUser{}, false, nil
	}
	var user AdminUser
	err := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).
		Where("oidc_subject = ?", subject).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminUser{}, false, nil
	}
	if err != nil {
		return AdminUser{}, false, fmt.Errorf("load oidc user: %w", err)
	}
	return user, true, nil
}

func (service *Service) BindOIDC(ctx context.Context, username string, subject string, oidcUsername string, email string) error {
	if service.db == nil {
		return fmt.Errorf("database-backed admin login is not available")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("oidc subject is required")
	}
	current, found, err := service.UserByOIDCSubject(ctx, subject)
	if err != nil {
		return err
	}
	if found && current.Username != username {
		return fmt.Errorf("this Keycloak account is already bound to another admin")
	}
	now := time.Now()
	updates := map[string]any{
		"oidc_subject":  subject,
		"oidc_username": stringPtr(oidcUsername),
		"oidc_email":    stringPtr(email),
		"oidc_bound_at": now,
		"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
	}
	result := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).Where("username = ?", username).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("bind oidc account: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}
	return nil
}

func (service *Service) UnbindOIDC(ctx context.Context, username string) error {
	if service.db == nil {
		return fmt.Errorf("database-backed admin login is not available")
	}
	result := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).Where("username = ?", username).Updates(map[string]any{
		"oidc_subject":  nil,
		"oidc_username": nil,
		"oidc_email":    nil,
		"oidc_bound_at": nil,
		"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if result.Error != nil {
		return fmt.Errorf("unbind oidc account: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}
	return nil
}

func EnsureInitialAdmin(ctx context.Context, dbConn *gorm.DB, username string) (string, bool, error) {
	var count int64
	if err := dbConn.WithContext(ctx).Model(&appdb.AdminUser{}).Count(&count).Error; err != nil {
		return "", false, fmt.Errorf("count admin users: %w", err)
	}
	if count > 0 {
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

	if err := dbConn.WithContext(ctx).Create(&appdb.AdminUser{Username: username, PasswordHash: passwordHash}).Error; err != nil {
		return "", false, fmt.Errorf("create initial admin: %w", err)
	}

	return password, true, nil
}

func (service *Service) ChangePassword(ctx context.Context, username string, currentPassword string, newPassword string, currentSessionID string) error {
	if service.db == nil {
		return fmt.Errorf("database-backed admin login is not available")
	}
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("new password is required")
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	var passwordHash string
	err := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).Select("password_hash").Where("username = ?", username).Take(&passwordHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("admin user not found")
	}
	if err != nil {
		return fmt.Errorf("load admin user: %w", err)
	}

	ok, err := VerifyPassword(currentPassword, passwordHash)
	if err != nil || !ok {
		return fmt.Errorf("current password is incorrect")
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	if err := service.db.WithContext(ctx).Model(&appdb.AdminUser{}).Where("username = ?", username).Updates(map[string]any{
		"password_hash": newHash,
		"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
	}).Error; err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	subquery := service.db.Model(&appdb.AdminUser{}).Select("id").Where("username = ?", username).Limit(1)
	if err := service.db.WithContext(ctx).Where("admin_user_id = (?) AND id <> ?", subquery, currentSessionID).Delete(&appdb.Session{}).Error; err != nil {
		return fmt.Errorf("invalidate other sessions: %w", err)
	}

	return nil
}

func ResetAdminPassword(ctx context.Context, dbConn *gorm.DB, username string, password string) (string, error) {
	if strings.TrimSpace(username) == "" {
		return "", fmt.Errorf("admin username is required")
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

	result := dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "username"}},
		DoUpdates: clause.Assignments(map[string]any{
			"password_hash": passwordHash,
			"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(&appdb.AdminUser{Username: username, PasswordHash: passwordHash})
	if result.Error != nil {
		return "", fmt.Errorf("reset admin password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return "", fmt.Errorf("admin password was not updated")
	}

	subquery := dbConn.Model(&appdb.AdminUser{}).Select("id").Where("username = ?", username).Limit(1)
	if err := dbConn.WithContext(ctx).Where("admin_user_id = (?)", subquery).Delete(&appdb.Session{}).Error; err != nil {
		return "", fmt.Errorf("invalidate sessions after password reset: %w", err)
	}

	return password, nil
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
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
