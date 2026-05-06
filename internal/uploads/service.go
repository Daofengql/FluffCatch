package uploads

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/storage"
)

type Service struct {
	db             *sql.DB
	storageManager *storage.Manager
}

type FileUpload struct {
	File               multipart.File
	Header             *multipart.FileHeader
	SubmissionPassword string
	PhotographerName   string
	Tags               []string
}

func NewService(dbConn *sql.DB, storageManager *storage.Manager) *Service {
	return &Service{
		db:             dbConn,
		storageManager: storageManager,
	}
}

func (service *Service) Create(ctx context.Context, eventID int64, upload FileUpload) (Submission, error) {
	if service.db == nil {
		return Submission{}, fmt.Errorf("database is required")
	}

	if err := service.verifySubmissionPassword(ctx, eventID, upload.SubmissionPassword); err != nil {
		return Submission{}, err
	}

	store, err := service.storageManager.ActiveStore()
	if err != nil {
		return Submission{}, err
	}

	objectKey, err := submissionObjectKey(eventID, upload.Header.Filename)
	if err != nil {
		return Submission{}, err
	}

	stored, err := store.Put(ctx, storage.Object{
		Key:         objectKey,
		Content:     upload.File,
		ContentType: upload.Header.Header.Get("Content-Type"),
		Size:        upload.Header.Size,
	})
	if err != nil {
		return Submission{}, err
	}

	tags := normalizeTags(upload.Tags)
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return Submission{}, fmt.Errorf("encode tags: %w", err)
	}

	result, err := service.db.ExecContext(ctx, `
		INSERT INTO submissions (
			event_id, storage_policy_id, object_key, original_filename,
			content_type, size_bytes, photographer_name, tags, status
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')
	`, eventID, stored.PolicyID, stored.Key, upload.Header.Filename, upload.Header.Header.Get("Content-Type"), upload.Header.Size, nullableString(upload.PhotographerName), string(tagsJSON))
	if err != nil {
		_ = store.Delete(ctx, stored.Key)
		return Submission{}, fmt.Errorf("create submission: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Submission{}, fmt.Errorf("read submission id: %w", err)
	}

	created, found, err := service.Get(ctx, id)
	if err != nil {
		return Submission{}, err
	}
	if !found {
		return Submission{}, fmt.Errorf("created submission not found")
	}

	return created, nil
}

func (service *Service) Get(ctx context.Context, id int64) (Submission, bool, error) {
	row := service.db.QueryRowContext(ctx, `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			original_filename, content_type, size_bytes, COALESCE(photographer_name, ''),
			tags, status, created_at
		FROM submissions
		WHERE id = ?
		LIMIT 1
	`, id)

	submission, err := scanSubmission(row)
	if err == sql.ErrNoRows {
		return Submission{}, false, nil
	}
	if err != nil {
		return Submission{}, false, err
	}

	return submission, true, nil
}

func (service *Service) ListPending(ctx context.Context) ([]Submission, error) {
	if service.db == nil {
		return []Submission{}, nil
	}

	rows, err := service.db.QueryContext(ctx, `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			original_filename, content_type, size_bytes, COALESCE(photographer_name, ''),
			tags, status, created_at
		FROM submissions
		WHERE status = 'pending'
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending submissions: %w", err)
	}
	defer rows.Close()

	submissions := []Submission{}
	for rows.Next() {
		submission, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		submissions = append(submissions, submission)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate submissions: %w", err)
	}

	return submissions, nil
}

func (service *Service) ApproveBatch(ctx context.Context, ids []int64) (BatchResponse, error) {
	processed := 0
	for _, id := range ids {
		ok, err := service.approveOne(ctx, id)
		if err != nil {
			return BatchResponse{}, err
		}
		if ok {
			processed++
		}
	}

	return BatchResponse{Processed: processed, Message: "submissions approved"}, nil
}

func (service *Service) DeleteBatch(ctx context.Context, ids []int64) (BatchResponse, error) {
	processed := 0
	for _, id := range ids {
		ok, err := service.deleteOne(ctx, id)
		if err != nil {
			return BatchResponse{}, err
		}
		if ok {
			processed++
		}
	}

	return BatchResponse{Processed: processed, Message: "submissions deleted"}, nil
}

func (service *Service) verifySubmissionPassword(ctx context.Context, eventID int64, password string) error {
	var enabled bool
	var passwordHash sql.NullString
	err := service.db.QueryRowContext(ctx, `
		SELECT submission_enabled, submission_password_hash
		FROM events
		WHERE id = ?
		LIMIT 1
	`, eventID).Scan(&enabled, &passwordHash)
	if err == sql.ErrNoRows {
		return fmt.Errorf("event not found")
	}
	if err != nil {
		return fmt.Errorf("load event submission settings: %w", err)
	}
	if !enabled {
		return fmt.Errorf("submissions are closed")
	}
	if !passwordHash.Valid || passwordHash.String == "" {
		return nil
	}

	ok, err := auth.VerifyPassword(password, passwordHash.String)
	if err != nil || !ok {
		return fmt.Errorf("invalid submission password")
	}

	return nil
}

func (service *Service) approveOne(ctx context.Context, id int64) (bool, error) {
	submission, found, err := service.Get(ctx, id)
	if err != nil {
		return false, err
	}
	if !found || submission.Status != SubmissionPending {
		return false, nil
	}

	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin approve submission: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO photos (
			event_id, storage_policy_id, object_key, thumbnail_key, original_filename,
			content_type, size_bytes, photographer_name, visibility
		)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), 'public')
	`, submission.EventID, submission.StoragePolicyID, submission.ObjectKey, submission.ThumbnailKey, submission.OriginalFilename, submission.ContentType, submission.SizeBytes, submission.PhotographerName)
	if err != nil {
		return false, fmt.Errorf("create photo from submission: %w", err)
	}

	photoID, err := result.LastInsertId()
	if err != nil {
		return false, fmt.Errorf("read photo id: %w", err)
	}

	if err := upsertPhotoTags(ctx, tx, photoID, submission.Tags); err != nil {
		return false, err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE submissions SET status = 'approved', approved_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
		return false, fmt.Errorf("mark submission approved: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit approve submission: %w", err)
	}

	return true, nil
}

func (service *Service) deleteOne(ctx context.Context, id int64) (bool, error) {
	submission, found, err := service.Get(ctx, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	store, err := service.storageManager.StoreForPolicy(submission.StoragePolicyID)
	if err == nil {
		_ = store.Delete(ctx, submission.ObjectKey)
	}

	if _, err := service.db.ExecContext(ctx, "DELETE FROM submissions WHERE id = ?", id); err != nil {
		return false, fmt.Errorf("delete submission: %w", err)
	}

	return true, nil
}

type submissionScanner interface {
	Scan(dest ...any) error
}

func scanSubmission(scanner submissionScanner) (Submission, error) {
	var submission Submission
	var tagsRaw []byte
	var status string

	if err := scanner.Scan(
		&submission.ID,
		&submission.EventID,
		&submission.StoragePolicyID,
		&submission.ObjectKey,
		&submission.ThumbnailKey,
		&submission.OriginalFilename,
		&submission.ContentType,
		&submission.SizeBytes,
		&submission.PhotographerName,
		&tagsRaw,
		&status,
		&submission.CreatedAt,
	); err != nil {
		return Submission{}, err
	}

	submission.Status = SubmissionStatus(status)
	if len(tagsRaw) > 0 {
		_ = json.Unmarshal(tagsRaw, &submission.Tags)
	}

	return submission, nil
}

func upsertPhotoTags(ctx context.Context, tx *sql.Tx, photoID int64, tags []string) error {
	for _, tag := range normalizeTags(tags) {
		result, err := tx.ExecContext(ctx, "INSERT INTO tags (name) VALUES (?) ON DUPLICATE KEY UPDATE name = VALUES(name)", tag)
		if err != nil {
			return fmt.Errorf("upsert tag: %w", err)
		}

		tagID, err := result.LastInsertId()
		if err != nil || tagID == 0 {
			if err := tx.QueryRowContext(ctx, "SELECT id FROM tags WHERE name = ? LIMIT 1", tag).Scan(&tagID); err != nil {
				return fmt.Errorf("load tag id: %w", err)
			}
		}

		if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO photo_tags (photo_id, tag_id) VALUES (?, ?)", photoID, tagID); err != nil {
			return fmt.Errorf("link photo tag: %w", err)
		}
	}

	return nil
}

func submissionObjectKey(eventID int64, filename string) (string, error) {
	token, err := randomHex(8)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}

	return fmt.Sprintf("events/%d/submissions/%d-%s%s", eventID, time.Now().UnixNano(), token, ext), nil
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		tag = strings.TrimPrefix(tag, "#")
		if tag == "" {
			continue
		}
		tag = "#" + tag
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

func randomHex(bytesLength int) (string, error) {
	raw := make([]byte, bytesLength)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
