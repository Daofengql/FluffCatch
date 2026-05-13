package auth

import "time"

type AdminUser struct {
	Username string `json:"username"`
}

type Session struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
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
	Enabled      bool   `json:"enabled"`
	Bound        bool   `json:"bound"`
	Subject      string `json:"subject,omitempty"`
	ProviderName string `json:"providerName,omitempty"`
}
