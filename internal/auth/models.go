package auth

import "time"

type AdminUser struct {
	ID           int64      `gorm:"column:id" json:"id"`
	Username     string     `gorm:"column:username" json:"username"`
	PasswordHash string     `gorm:"column:password_hash" json:"-"`
	OIDCSubject  string     `gorm:"column:oidc_subject" json:"oidcSubject,omitempty"`
	OIDCUsername string     `gorm:"column:oidc_username" json:"oidcUsername,omitempty"`
	OIDCEmail    string     `gorm:"column:oidc_email" json:"oidcEmail,omitempty"`
	OIDCBoundAt  *time.Time `gorm:"column:oidc_bound_at" json:"oidcBoundAt,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type LoginRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

type LoginResponse struct {
	Authenticated bool   `json:"authenticated"`
	Message       string `json:"message"`
	Username      string `json:"username,omitempty"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type MeResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
}

type OIDCStatus struct {
	Enabled      bool       `json:"enabled"`
	Bound        bool       `json:"bound"`
	Subject      string     `json:"subject,omitempty"`
	Username     string     `json:"username,omitempty"`
	Email        string     `json:"email,omitempty"`
	BoundAt      *time.Time `json:"boundAt,omitempty"`
	ProviderName string     `json:"providerName,omitempty"`
}
