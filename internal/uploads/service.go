package uploads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/gallery"
	appimage "fluffcatch/internal/image"
	"fluffcatch/internal/storage"
)

type Service struct {
	db             *sql.DB
	storageManager *storage.Manager
	maxSizeBytes   int64
}

type FileUpload struct {
	File               multipart.File
	Header             *multipart.FileHeader
	SubmissionPassword string
	PhotographerName   string
	Tags               []string
}

type storedUpload struct {
	PolicyID     string
	ObjectKey    string
	ThumbnailKey string
	ContentHash  string
	ContentType  string
	SizeBytes    int64
}

func NewService(dbConn *sql.DB, storageManager *storage.Manager, maxSizeMB int) *Service {
	return &Service{
		db:             dbConn,
		storageManager: storageManager,
		maxSizeBytes:   int64(maxSizeMB) * 1024 * 1024,
	}
}

func (service *Service) Create(ctx context.Context, eventID int64, upload FileUpload) (Submission, error) {
	if service.db == nil {
		return Submission{}, fmt.Errorf("database is required")
	}

	if err := service.verifySubmissionPassword(ctx, eventID, upload.SubmissionPassword); err != nil {
		return Submission{}, err
	}
	storedUpload, store, err := service.storeUpload(ctx, eventID, upload)
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
			event_id, storage_policy_id, object_key, thumbnail_key, content_hash,
			content_type, size_bytes, photographer_name, tags, status
		)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, 'pending')
	`, eventID, storedUpload.PolicyID, storedUpload.ObjectKey, storedUpload.ThumbnailKey, storedUpload.ContentHash, storedUpload.ContentType, storedUpload.SizeBytes, nullableString(upload.PhotographerName), string(tagsJSON))
	if err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
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

func (service *Service) CreateApproved(ctx context.Context, eventID int64, upload FileUpload) (gallery.Photo, error) {
	if service.db == nil {
		return gallery.Photo{}, fmt.Errorf("database is required")
	}

	if err := service.verifyEventAllowsSubmission(ctx, eventID); err != nil {
		return gallery.Photo{}, err
	}
	storedUpload, store, err := service.storeUpload(ctx, eventID, upload)
	if err != nil {
		return gallery.Photo{}, err
	}

	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
		return gallery.Photo{}, fmt.Errorf("begin create approved photo: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO photos (
			event_id, storage_policy_id, object_key, thumbnail_key, content_hash,
			content_type, size_bytes, photographer_name, visibility
		)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), 'public')
	`, eventID, storedUpload.PolicyID, storedUpload.ObjectKey, storedUpload.ThumbnailKey, storedUpload.ContentHash, storedUpload.ContentType, storedUpload.SizeBytes, strings.TrimSpace(upload.PhotographerName))
	if err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
		return gallery.Photo{}, fmt.Errorf("create approved photo: %w", err)
	}

	photoID, err := result.LastInsertId()
	if err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
		return gallery.Photo{}, fmt.Errorf("read photo id: %w", err)
	}

	if err := upsertPhotoTags(ctx, tx, photoID, upload.Tags); err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
		return gallery.Photo{}, err
	}

	if err := tx.Commit(); err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
		return gallery.Photo{}, fmt.Errorf("commit create approved photo: %w", err)
	}

	photo, err := service.photoByID(ctx, photoID)
	if err != nil {
		return gallery.Photo{}, err
	}
	return photo, nil
}

func (service *Service) storeUpload(ctx context.Context, eventID int64, upload FileUpload) (storedUpload, storage.Store, error) {
	if upload.Header.Size > service.maxSizeBytes {
		return storedUpload{}, nil, fmt.Errorf("file exceeds maximum upload size of %d MB", service.maxSizeBytes/(1024*1024))
	}

	limited := io.LimitReader(upload.File, service.maxSizeBytes+1)

	// Detect actual content type from magic bytes.
	head := make([]byte, 512)
	n, err := io.ReadFull(limited, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return storedUpload{}, nil, fmt.Errorf("read upload file: %w", err)
	}
	head = head[:n]
	detectedType := http.DetectContentType(head)
	if !strings.HasPrefix(detectedType, "image/") {
		return storedUpload{}, nil, fmt.Errorf("file is not a supported image format")
	}

	content, err := io.ReadAll(io.MultiReader(bytes.NewReader(head), limited))
	if err != nil {
		return storedUpload{}, nil, fmt.Errorf("read upload file: %w", err)
	}
	if len(content) == 0 {
		return storedUpload{}, nil, fmt.Errorf("empty file is not allowed")
	}
	if int64(len(content)) > service.maxSizeBytes {
		return storedUpload{}, nil, fmt.Errorf("file exceeds maximum upload size of %d MB", service.maxSizeBytes/(1024*1024))
	}

	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])
	if exists, err := service.eventHashExists(ctx, eventID, contentHash); err != nil {
		return storedUpload{}, nil, err
	} else if exists {
		return storedUpload{}, nil, fmt.Errorf("duplicate image already exists in this event")
	}

	store, err := service.storageManager.ActiveStore()
	if err != nil {
		return storedUpload{}, nil, err
	}

	contentType := strings.TrimSpace(upload.Header.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	objectKey := imageObjectKey(eventID, contentHash, upload.Header.Filename)
	thumbnailKey := thumbnailObjectKey(eventID, contentHash)

	stored, err := store.Put(ctx, storage.Object{
		Key:         objectKey,
		Content:     bytes.NewReader(content),
		ContentType: contentType,
		Size:        int64(len(content)),
	})
	if err != nil {
		return storedUpload{}, nil, err
	}

	var storedThumbnail storage.StoredObject
	thumbnail, thumbnailContentType, err := appimage.GenerateThumbnailBytes(content, 640)
	if err == nil && len(thumbnail) > 0 {
		storedThumbnail, err = store.Put(ctx, storage.Object{
			Key:         thumbnailKey,
			Content:     bytes.NewReader(thumbnail),
			ContentType: thumbnailContentType,
			Size:        int64(len(thumbnail)),
		})
		if err != nil {
			_ = store.Delete(ctx, stored.Key)
			return storedUpload{}, nil, fmt.Errorf("store thumbnail: %w", err)
		}
	} else {
		thumbnailKey = ""
	}

	return storedUpload{
		PolicyID:     stored.PolicyID,
		ObjectKey:    stored.Key,
		ThumbnailKey: storedThumbnail.Key,
		ContentHash:  contentHash,
		ContentType:  contentType,
		SizeBytes:    int64(len(content)),
	}, store, nil
}

func (service *Service) deleteStoredUpload(ctx context.Context, store storage.Store, upload storedUpload) {
	if store == nil {
		return
	}
	_ = store.Delete(ctx, upload.ObjectKey)
	if upload.ThumbnailKey != "" {
		_ = store.Delete(ctx, upload.ThumbnailKey)
	}
}

func (service *Service) eventHashExists(ctx context.Context, eventID int64, contentHash string) (bool, error) {
	var exists int
	err := service.db.QueryRowContext(ctx, `
		SELECT 1
		FROM (
			SELECT content_hash FROM photos WHERE event_id = ? AND content_hash = ?
			UNION ALL
			SELECT content_hash FROM submissions WHERE event_id = ? AND content_hash = ? AND status = 'pending'
		) AS image_hashes
		LIMIT 1
	`, eventID, contentHash, eventID, contentHash).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check duplicate image: %w", err)
	}
	return true, nil
}

func (service *Service) photoByID(ctx context.Context, photoID int64) (gallery.Photo, error) {
	row := service.db.QueryRowContext(ctx, `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			content_hash, content_type, size_bytes, like_count, COALESCE(photographer_name, ''), visibility, taken_at, created_at, updated_at
		FROM photos
		WHERE id = ?
		LIMIT 1
	`, photoID)

	photo, err := scanPhoto(row, service.storageManager)
	if err != nil {
		return gallery.Photo{}, err
	}

	photos := []gallery.Photo{photo}
	if err := attachPhotoTags(ctx, service.db, photos); err != nil {
		return gallery.Photo{}, err
	}
	return photos[0], nil
}

func (service *Service) Get(ctx context.Context, id int64) (Submission, bool, error) {
	row := service.db.QueryRowContext(ctx, `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			content_hash, content_type, size_bytes, COALESCE(photographer_name, ''),
			tags, status, created_at
		FROM submissions
		WHERE id = ?
		LIMIT 1
	`, id)

	submission, err := service.scanSubmission(row)
	if err == sql.ErrNoRows {
		return Submission{}, false, nil
	}
	if err != nil {
		return Submission{}, false, err
	}

	return submission, true, nil
}

func (service *Service) ListPending(ctx context.Context) ([]Submission, error) {
	return service.ListPendingForEvent(ctx, 0)
}

func (service *Service) ListPendingForEvent(ctx context.Context, eventID int64) ([]Submission, error) {
	if service.db == nil {
		return []Submission{}, nil
	}

	where := "WHERE status = 'pending'"
	args := []any{}
	if eventID > 0 {
		where += " AND event_id = ?"
		args = append(args, eventID)
	}

	rows, err := service.db.QueryContext(ctx, `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			content_hash, content_type, size_bytes, COALESCE(photographer_name, ''),
			tags, status, created_at
		FROM submissions
		`+where+`
		ORDER BY created_at DESC, id DESC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list pending submissions: %w", err)
	}
	defer rows.Close()

	submissions := []Submission{}
	for rows.Next() {
		submission, err := service.scanSubmission(rows)
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

func (service *Service) verifyEventAllowsSubmission(ctx context.Context, eventID int64) error {
	var enabled bool
	err := service.db.QueryRowContext(ctx, `
		SELECT submission_enabled
		FROM events
		WHERE id = ?
		LIMIT 1
	`, eventID).Scan(&enabled)
	if err == sql.ErrNoRows {
		return fmt.Errorf("event not found")
	}
	if err != nil {
		return fmt.Errorf("load event submission settings: %w", err)
	}
	if !enabled {
		return fmt.Errorf("submissions are closed")
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
			event_id, storage_policy_id, object_key, thumbnail_key, content_hash,
			content_type, size_bytes, photographer_name, visibility
		)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), 'public')
	`, submission.EventID, submission.StoragePolicyID, submission.ObjectKey, submission.ThumbnailKey, submission.ContentHash, submission.ContentType, submission.SizeBytes, submission.PhotographerName)
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

	if _, err := tx.ExecContext(ctx, "DELETE FROM submissions WHERE id = ?", id); err != nil {
		return false, fmt.Errorf("remove approved submission: %w", err)
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
		if submission.ThumbnailKey != "" {
			_ = store.Delete(ctx, submission.ThumbnailKey)
		}
	}

	if _, err := service.db.ExecContext(ctx, "DELETE FROM submissions WHERE id = ?", id); err != nil {
		return false, fmt.Errorf("delete submission: %w", err)
	}

	return true, nil
}

type submissionScanner interface {
	Scan(dest ...any) error
}

func (service *Service) scanSubmission(scanner submissionScanner) (Submission, error) {
	var submission Submission
	var tagsRaw []byte
	var status string

	if err := scanner.Scan(
		&submission.ID,
		&submission.EventID,
		&submission.StoragePolicyID,
		&submission.ObjectKey,
		&submission.ThumbnailKey,
		&submission.ContentHash,
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
	if store, err := service.storageManager.StoreForPolicy(submission.StoragePolicyID); err == nil {
		submission.URL = store.PublicURL(submission.ObjectKey)
		if submission.ThumbnailKey != "" {
			submission.ThumbnailURL = store.PublicURL(submission.ThumbnailKey)
		}
	} else {
		submission.URL = storage.MediaURL(submission.StoragePolicyID, submission.ObjectKey)
	}
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

type photoScanner interface {
	Scan(dest ...any) error
}

func scanPhoto(scanner photoScanner, storageManager *storage.Manager) (gallery.Photo, error) {
	var photo gallery.Photo
	var thumbnailKey string
	var takenAt sql.NullTime
	var visibility string

	if err := scanner.Scan(
		&photo.ID,
		&photo.EventID,
		&photo.StoragePolicyID,
		&photo.ObjectKey,
		&thumbnailKey,
		&photo.ContentHash,
		&photo.ContentType,
		&photo.SizeBytes,
		&photo.LikeCount,
		&photo.PhotographerName,
		&visibility,
		&takenAt,
		&photo.CreatedAt,
		&photo.UpdatedAt,
	); err != nil {
		return gallery.Photo{}, err
	}

	photo.Tags = []gallery.Tag{}
	photo.Visibility = gallery.Visibility(visibility)
	photo.ThumbnailKey = thumbnailKey
	if takenAt.Valid {
		photo.TakenAt = &takenAt.Time
	}

	if store, err := storageManager.StoreForPolicy(photo.StoragePolicyID); err == nil {
		photo.URL = store.PublicURL(photo.ObjectKey)
		if photo.ThumbnailKey != "" {
			photo.ThumbnailURL = store.PublicURL(photo.ThumbnailKey)
		}
	} else {
		photo.URL = storage.MediaURL(photo.StoragePolicyID, photo.ObjectKey)
	}

	return photo, nil
}

func attachPhotoTags(ctx context.Context, dbConn *sql.DB, photos []gallery.Photo) error {
	if len(photos) == 0 {
		return nil
	}

	ids := make([]string, len(photos))
	args := make([]any, len(photos))
	indexByID := map[int64]int{}
	for index, photo := range photos {
		ids[index] = "?"
		args[index] = photo.ID
		indexByID[photo.ID] = index
	}

	rows, err := dbConn.QueryContext(ctx, `
		SELECT photo_tags.photo_id, tags.id, tags.name, tags.created_at
		FROM photo_tags
		INNER JOIN tags ON tags.id = photo_tags.tag_id
		WHERE photo_tags.photo_id IN (`+strings.Join(ids, ",")+`)
		ORDER BY tags.name
	`, args...)
	if err != nil {
		return fmt.Errorf("load photo tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var photoID int64
		var tag gallery.Tag
		if err := rows.Scan(&photoID, &tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return fmt.Errorf("scan photo tag: %w", err)
		}
		if index, ok := indexByID[photoID]; ok {
			photos[index].Tags = append(photos[index].Tags, tag)
		}
	}

	return rows.Err()
}

func imageObjectKey(eventID int64, contentHash string, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("events/%d/images/%s%s", eventID, contentHash, ext)
}

func thumbnailObjectKey(eventID int64, contentHash string) string {
	return fmt.Sprintf("events/%d/thumbnails/%s.jpg", eventID, contentHash)
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

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
