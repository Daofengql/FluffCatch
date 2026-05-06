package auth

import "time"

type AdminUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
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

type MeResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
}
