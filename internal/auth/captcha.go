package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"html"
	"math/big"
	"strings"
	"sync"
	"time"
)

const captchaTTL = 5 * time.Minute

type CaptchaChallenge struct {
	ID        string `json:"id"`
	ImageSVG  string `json:"imageSvg"`
	ExpiresAt string `json:"expiresAt"`
}

type captchaEntry struct {
	answer    string
	expiresAt time.Time
}

type CaptchaStore struct {
	mu      sync.Mutex
	entries map[string]captchaEntry
}

func NewCaptchaStore() *CaptchaStore {
	return &CaptchaStore{entries: map[string]captchaEntry{}}
}

func (store *CaptchaStore) NewChallenge(ctx context.Context) (CaptchaChallenge, error) {
	_ = ctx

	id, err := randomHex(16)
	if err != nil {
		return CaptchaChallenge{}, err
	}

	answer, err := randomDigits(6)
	if err != nil {
		return CaptchaChallenge{}, err
	}

	expiresAt := time.Now().Add(captchaTTL)
	store.mu.Lock()
	store.cleanupLocked(time.Now())
	store.entries[id] = captchaEntry{answer: answer, expiresAt: expiresAt}
	store.mu.Unlock()

	return CaptchaChallenge{
		ID:        id,
		ImageSVG:  captchaSVG(answer),
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

func (store *CaptchaStore) Verify(ctx context.Context, id string, answer string) bool {
	_ = ctx
	id = strings.TrimSpace(id)
	answer = strings.TrimSpace(answer)
	if id == "" || answer == "" {
		return false
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	store.cleanupLocked(time.Now())

	entry, ok := store.entries[id]
	if !ok {
		return false
	}
	delete(store.entries, id)

	return strings.EqualFold(entry.answer, answer)
}

func (store *CaptchaStore) cleanupLocked(now time.Time) {
	for id, entry := range store.entries {
		if now.After(entry.expiresAt) {
			delete(store.entries, id)
		}
	}
}

func randomDigits(length int) (string, error) {
	var builder strings.Builder
	for i := 0; i < length; i++ {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generate captcha digit: %w", err)
		}
		builder.WriteString(value.String())
	}
	return builder.String(), nil
}

func captchaSVG(answer string) string {
	escaped := html.EscapeString(answer)
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="180" height="54" viewBox="0 0 180 54">
  <rect width="180" height="54" rx="12" fill="#e3f2fd"/>
  <path d="M8 42 C32 10, 52 60, 82 20 S124 44, 142 12" fill="none" stroke="#90caf9" stroke-width="3"/>
  <path d="M14 16 L168 39 M20 43 L164 12" stroke="#bbdefb" stroke-width="2"/>
  <text x="50%%" y="50%%" dominant-baseline="middle" text-anchor="middle" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="26" font-weight="800" letter-spacing="6" fill="#1565c0">%s</text>
</svg>`, escaped)
}
