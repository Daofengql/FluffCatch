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
	colors := []string{"#1565c0", "#c62828", "#2e7d32", "#6a1b9a", "#e65100", "#00838f"}
	bgColors := []string{"#e3f2fd", "#ffebee", "#e8f5e9", "#f3e5f5", "#fff3e0", "#e0f7fa"}
	r := randInt(0, len(colors))
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="60" viewBox="0 0 200 60">
  <rect width="200" height="60" rx="12" fill="%s"/>
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="%d" opacity="0.5"/>
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="%d" opacity="0.4"/>
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="%d" opacity="0.4"/>
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="%d" opacity="0.3"/>
  <text x="50%%" y="50%%" dominant-baseline="middle" text-anchor="middle" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="28" font-weight="700" letter-spacing="4" fill="%s" transform="rotate(%d, 100, 30)">%s</text>
</svg>`,
		bgColors[r],
		randInt(0, 50), randInt(0, 30), randInt(150, 200), randInt(30, 60), colors[(r+1)%len(colors)], randInt(1, 3),
		randInt(0, 200), randInt(40, 60), randInt(30, 170), randInt(0, 20), colors[(r+2)%len(colors)], randInt(1, 3),
		randInt(10, 60), randInt(0, 10), randInt(140, 190), randInt(50, 60), colors[(r+3)%len(colors)], randInt(1, 3),
		randInt(50, 150), randInt(0, 60), randInt(60, 140), randInt(0, 60), colors[(r+4)%len(colors)], randInt(1, 3),
		colors[r],
		randInt(-3, 4),
		escaped,
	)
}

func randInt(min, max int) int {
	if min >= max {
		return min
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		return min
	}
	return min + int(n.Int64())
}
