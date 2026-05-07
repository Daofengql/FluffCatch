package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	passwordHashAlgorithm  = "pbkdf2-sha256"
	passwordHashIterations = 210000
	passwordSaltBytes      = 16
	generatedPasswordBytes = 18
)

type Service struct {
	db               *sql.DB
	fallbackUsername string
}

func NewService(dbConn *sql.DB, fallbackUsername string) *Service {
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
	err := service.db.QueryRowContext(ctx, "SELECT password_hash FROM admin_users WHERE username = ? LIMIT 1", req.Username).Scan(&passwordHash)
	if err == sql.ErrNoRows {
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
	if err := service.db.QueryRowContext(ctx, "SELECT id FROM admin_users WHERE username = ? LIMIT 1", username).Scan(&userID); err != nil {
		return "", time.Time{}, fmt.Errorf("load admin user for session: %w", err)
	}

	sessionID, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().Add(ttl)
	if _, err := service.db.ExecContext(ctx, "INSERT INTO sessions (id, admin_user_id, expires_at) VALUES (?, ?, ?)", sessionID, userID, expiresAt); err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}

	return sessionID, expiresAt, nil
}

func (service *Service) AuthenticateSession(ctx context.Context, sessionID string) (AdminUser, bool, error) {
	if service.db == nil || strings.TrimSpace(sessionID) == "" {
		return AdminUser{}, false, nil
	}

	var user AdminUser
	err := service.db.QueryRowContext(ctx, `
		SELECT admin_users.id, admin_users.username, admin_users.password_hash, admin_users.created_at, admin_users.updated_at
		FROM sessions
		INNER JOIN admin_users ON admin_users.id = sessions.admin_user_id
		WHERE sessions.id = ? AND sessions.expires_at > CURRENT_TIMESTAMP
		LIMIT 1
	`, sessionID).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
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

	if _, err := service.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func EnsureInitialAdmin(ctx context.Context, dbConn *sql.DB, username string) (string, bool, error) {
	var count int
	if err := dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_users").Scan(&count); err != nil {
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

	if _, err := dbConn.ExecContext(ctx, "INSERT INTO admin_users (username, password_hash) VALUES (?, ?)", username, passwordHash); err != nil {
		return "", false, fmt.Errorf("create initial admin: %w", err)
	}

	return password, true, nil
}

func ResetAdminPassword(ctx context.Context, dbConn *sql.DB, username string, password string) (string, error) {
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

	result, err := dbConn.ExecContext(ctx, `
		INSERT INTO admin_users (username, password_hash)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
			password_hash = VALUES(password_hash),
			updated_at = CURRENT_TIMESTAMP
	`, username, passwordHash)
	if err != nil {
		return "", fmt.Errorf("reset admin password: %w", err)
	}

	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return "", fmt.Errorf("admin password was not updated")
	}

	// Invalidate all sessions for this user after password reset.
	if _, err := dbConn.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE admin_user_id = (SELECT id FROM admin_users WHERE username = ? LIMIT 1)
	`, username); err != nil {
		return "", fmt.Errorf("invalidate sessions after password reset: %w", err)
	}

	return password, nil
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
