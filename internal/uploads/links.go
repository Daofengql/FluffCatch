package uploads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"fluffcatch/internal/auth"
	appdb "fluffcatch/internal/db"

	"gorm.io/gorm"
)

type SubmissionLink struct {
	ID               int64      `json:"id"`
	EventID          int64      `json:"eventId"`
	Label            string     `json:"label"`
	PhotographerName string     `json:"photographerName,omitempty"`
	Token            string     `json:"token,omitempty"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	MaxUses          int        `json:"maxUses"`
	UseCount         int        `json:"useCount"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type CreateSubmissionLinkRequest struct {
	Label            string `json:"label"`
	PhotographerName string `json:"photographerName"`
	ExpiresInHours   int    `json:"expiresInHours"`
	MaxUses          int    `json:"maxUses"`
}

func (service *Service) CreateSubmissionLink(ctx context.Context, eventID int64, req CreateSubmissionLinkRequest) (SubmissionLink, error) {
	if service.db == nil {
		return SubmissionLink{}, fmt.Errorf("database is required")
	}
	if err := service.verifyEventAllowsSubmission(ctx, eventID); err != nil {
		return SubmissionLink{}, err
	}
	req.Label = strings.TrimSpace(req.Label)
	req.PhotographerName = strings.TrimSpace(req.PhotographerName)
	if req.Label == "" {
		if req.PhotographerName != "" {
			req.Label = req.PhotographerName
		} else {
			req.Label = "投稿链接"
		}
	}
	if len(req.Label) > 191 {
		return SubmissionLink{}, fmt.Errorf("label is too long")
	}
	if len(req.PhotographerName) > 191 {
		return SubmissionLink{}, fmt.Errorf("photographer name is too long")
	}
	if req.ExpiresInHours < 0 {
		req.ExpiresInHours = 0
	}
	if req.ExpiresInHours > 24*365 {
		req.ExpiresInHours = 24 * 365
	}
	if req.MaxUses < 0 {
		req.MaxUses = 0
	}
	if req.MaxUses > 100000 {
		req.MaxUses = 100000
	}

	token, err := randomToken()
	if err != nil {
		return SubmissionLink{}, err
	}
	var expiresAt *time.Time
	if req.ExpiresInHours > 0 {
		value := time.Now().Add(time.Duration(req.ExpiresInHours) * time.Hour)
		expiresAt = &value
	}

	record := appdb.SubmissionLink{
		EventID:          eventID,
		TokenHash:        auth.TokenHash(token),
		Label:            req.Label,
		PhotographerName: stringPtr(req.PhotographerName),
		ExpiresAt:        expiresAt,
		MaxUses:          req.MaxUses,
	}
	if err := service.db.WithContext(ctx).Create(&record).Error; err != nil {
		return SubmissionLink{}, fmt.Errorf("create submission link: %w", err)
	}

	link := submissionLinkFromRecord(record)
	link.Token = token
	return link, nil
}

func (service *Service) ListSubmissionLinks(ctx context.Context, eventID int64) ([]SubmissionLink, error) {
	var records []appdb.SubmissionLink
	if err := service.db.WithContext(ctx).
		Where("event_id = ?", eventID).
		Order("created_at DESC").
		Order("id DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list submission links: %w", err)
	}
	links := make([]SubmissionLink, 0, len(records))
	for _, record := range records {
		links = append(links, submissionLinkFromRecord(record))
	}
	return links, nil
}

func (service *Service) RevokeSubmissionLink(ctx context.Context, eventID int64, linkID int64) (bool, error) {
	now := time.Now()
	result := service.db.WithContext(ctx).Model(&appdb.SubmissionLink{}).
		Where("id = ? AND event_id = ? AND revoked_at IS NULL", linkID, eventID).
		Update("revoked_at", now)
	if result.Error != nil {
		return false, fmt.Errorf("revoke submission link: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (service *Service) DeleteRevokedSubmissionLink(ctx context.Context, eventID int64, linkID int64) (bool, error) {
	result := service.db.WithContext(ctx).
		Where("id = ? AND event_id = ? AND revoked_at IS NOT NULL", linkID, eventID).
		Delete(&appdb.SubmissionLink{})
	if result.Error != nil {
		return false, fmt.Errorf("delete revoked submission link: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (service *Service) ResolveSubmissionToken(ctx context.Context, eventID int64, token string) (SubmissionLink, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return SubmissionLink{}, false, nil
	}
	if err := service.verifyEventAllowsSubmission(ctx, eventID); err != nil {
		return SubmissionLink{}, false, nil
	}
	var record appdb.SubmissionLink
	err := service.db.WithContext(ctx).
		Where("event_id = ? AND token_hash = ?", eventID, auth.TokenHash(token)).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmissionLink{}, false, nil
	}
	if err != nil {
		return SubmissionLink{}, false, fmt.Errorf("load submission link: %w", err)
	}
	now := time.Now()
	if record.RevokedAt != nil || (record.ExpiresAt != nil && now.After(*record.ExpiresAt)) || (record.MaxUses > 0 && record.UseCount >= record.MaxUses) {
		return SubmissionLink{}, false, nil
	}
	return submissionLinkFromRecord(record), true, nil
}

func submissionLinkFromRecord(record appdb.SubmissionLink) SubmissionLink {
	return SubmissionLink{
		ID:               record.ID,
		EventID:          record.EventID,
		Label:            record.Label,
		PhotographerName: stringValue(record.PhotographerName),
		ExpiresAt:        record.ExpiresAt,
		MaxUses:          record.MaxUses,
		UseCount:         record.UseCount,
		RevokedAt:        record.RevokedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func randomToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
