package auth

type AdminUser struct {
	Username string `json:"username"`
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
