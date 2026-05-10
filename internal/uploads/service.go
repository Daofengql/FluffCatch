package uploads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fluffcatch/internal/auth"
	appdb "fluffcatch/internal/db"
	"fluffcatch/internal/gallery"
	appimage "fluffcatch/internal/image"
	"fluffcatch/internal/storage"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db             *gorm.DB
	storageManager *storage.Manager
	maxImageBytes  int64
	maxVideoBytes  int64
}

type FileUpload struct {
	File               multipart.File
	Header             *multipart.FileHeader
	SubmissionToken    string
	PhotographerName   string
	Tags               []string
	Visibility         string
}

type storedUpload struct {
	PolicyID     string
	ObjectKey    string
	ThumbnailKey string
	ContentHash  string
	ContentType  string
	SizeBytes    int64
	Exif         []byte
	TakenAt      *time.Time
}

type bufferedUpload struct {
	File        *os.File
	SizeBytes   int64
	ContentHash string
	ContentType string
	IsImage     bool
}

type uploadLimits struct {
	MaxImageBytes int64
	MaxVideoBytes int64
}

func NewService(dbConn *gorm.DB, storageManager *storage.Manager, maxSizeMB int) *Service {
	return NewServiceWithLimits(dbConn, storageManager, maxSizeMB, maxSizeMB)
}

func NewServiceWithLimits(dbConn *gorm.DB, storageManager *storage.Manager, maxImageSizeMB int, maxVideoSizeMB int) *Service {
	return &Service{
		db:             dbConn,
		storageManager: storageManager,
		maxImageBytes:  int64(maxImageSizeMB) * 1024 * 1024,
		maxVideoBytes:  int64(maxVideoSizeMB) * 1024 * 1024,
	}
}

func (service *Service) Create(ctx context.Context, eventID int64, upload FileUpload) (Submission, error) {
	return service.CreateWithLimits(ctx, eventID, upload, service.maxImageBytes, service.maxVideoBytes)
}

func (service *Service) CreateWithLimit(ctx context.Context, eventID int64, upload FileUpload, maxSizeBytes int64) (Submission, error) {
	return service.CreateWithLimits(ctx, eventID, upload, maxSizeBytes, maxSizeBytes)
}

func (service *Service) CreateWithLimits(ctx context.Context, eventID int64, upload FileUpload, maxImageBytes int64, maxVideoBytes int64) (Submission, error) {
	if service.db == nil {
		return Submission{}, fmt.Errorf("database is required")
	}

	upload, err := service.applySubmissionAccess(ctx, eventID, upload)
	if err != nil {
		return Submission{}, err
	}
	storedUpload, store, err := service.storeUpload(ctx, eventID, upload, uploadLimits{MaxImageBytes: maxImageBytes, MaxVideoBytes: maxVideoBytes})
	if err != nil {
		return Submission{}, err
	}

	tags := normalizeTags(upload.Tags)
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return Submission{}, fmt.Errorf("encode tags: %w", err)
	}

	record := appdb.Submission{
		EventID:          eventID,
		StoragePolicyID:  storedUpload.PolicyID,
		ObjectKey:        storedUpload.ObjectKey,
		ThumbnailKey:     stringPtr(storedUpload.ThumbnailKey),
		ContentHash:      storedUpload.ContentHash,
		ContentType:      storedUpload.ContentType,
		SizeBytes:        storedUpload.SizeBytes,
		PhotographerName: stringPtr(upload.PhotographerName),
		Tags:             tagsJSON,
		Status:           string(SubmissionPending),
		Exif:             storedUpload.Exif,
		TakenAt:          storedUpload.TakenAt,
	}
	err = service.db.WithContext(ctx).Create(&record).Error
	if err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
		return Submission{}, fmt.Errorf("create submission: %w", err)
	}

	created, found, err := service.Get(ctx, record.ID)
	if err != nil {
		return Submission{}, err
	}
	if !found {
		return Submission{}, fmt.Errorf("created submission not found")
	}

	return created, nil
}

func visibilityOrDefault(v string) string {
	if v == string(gallery.VisibilityPublic) || v == string(gallery.VisibilityPrivate) {
		return v
	}
	return string(gallery.VisibilityPublic)
}

func (service *Service) CreateApproved(ctx context.Context, eventID int64, upload FileUpload) (gallery.Photo, error) {
	return service.CreateApprovedWithLimits(ctx, eventID, upload, service.maxImageBytes, service.maxVideoBytes)
}

func (service *Service) CreateApprovedWithLimit(ctx context.Context, eventID int64, upload FileUpload, maxSizeBytes int64) (gallery.Photo, error) {
	return service.CreateApprovedWithLimits(ctx, eventID, upload, maxSizeBytes, maxSizeBytes)
}

func (service *Service) CreateApprovedWithLimits(ctx context.Context, eventID int64, upload FileUpload, maxImageBytes int64, maxVideoBytes int64) (gallery.Photo, error) {
	if service.db == nil {
		return gallery.Photo{}, fmt.Errorf("database is required")
	}

	if err := service.verifyEventAllowsSubmission(ctx, eventID); err != nil {
		return gallery.Photo{}, err
	}
	storedUpload, store, err := service.storeUpload(ctx, eventID, upload, uploadLimits{MaxImageBytes: maxImageBytes, MaxVideoBytes: maxVideoBytes})
	if err != nil {
		return gallery.Photo{}, err
	}

	visibility := visibilityOrDefault(upload.Visibility)

	var photoID int64
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := appdb.Photo{
			EventID:          eventID,
			StoragePolicyID:  storedUpload.PolicyID,
			ObjectKey:        storedUpload.ObjectKey,
			ThumbnailKey:     stringPtr(storedUpload.ThumbnailKey),
			ContentHash:      storedUpload.ContentHash,
			ContentType:      storedUpload.ContentType,
			SizeBytes:        storedUpload.SizeBytes,
			PhotographerName: stringPtr(upload.PhotographerName),
			Visibility:       visibility,
			Exif:             storedUpload.Exif,
			TakenAt:          storedUpload.TakenAt,
			SortAt:           time.Now(),
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("create approved photo: %w", err)
		}
		photoID = record.ID
		if err := upsertPhotoTags(ctx, tx, photoID, upload.Tags); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		service.deleteStoredUpload(ctx, store, storedUpload)
		return gallery.Photo{}, err
	}

	photo, err := service.photoByID(ctx, photoID)
	if err != nil {
		return gallery.Photo{}, err
	}
	return photo, nil
}

func (service *Service) storeUpload(ctx context.Context, eventID int64, upload FileUpload, limits uploadLimits) (storedUpload, storage.Store, error) {
	if upload.Header == nil || upload.File == nil {
		return storedUpload{}, nil, fmt.Errorf("file is required")
	}
	if limits.MaxImageBytes <= 0 {
		limits.MaxImageBytes = 20 * 1024 * 1024
	}
	if limits.MaxVideoBytes <= 0 {
		limits.MaxVideoBytes = limits.MaxImageBytes
	}
	maxPossibleBytes := max(limits.MaxImageBytes, limits.MaxVideoBytes)
	if upload.Header.Size > maxPossibleBytes {
		return storedUpload{}, nil, fmt.Errorf("file exceeds maximum upload size of %d MB", maxPossibleBytes/(1024*1024))
	}

	buffered, err := bufferUploadToTemp(upload.File, maxPossibleBytes)
	if err != nil {
		return storedUpload{}, nil, err
	}
	defer func() {
		name := buffered.File.Name()
		_ = buffered.File.Close()
		_ = os.Remove(name)
	}()

	if !isAllowedMediaContentType(buffered.ContentType) {
		return storedUpload{}, nil, fmt.Errorf("file is not a supported image or video format")
	}
	if limit := limitForContentType(buffered.ContentType, limits); limit > 0 && buffered.SizeBytes > limit {
		return storedUpload{}, nil, fmt.Errorf("%s exceeds maximum upload size of %d MB", mediaKind(buffered.ContentType), limit/(1024*1024))
	}

	if exists, err := service.eventHashExists(ctx, eventID, buffered.ContentHash); err != nil {
		return storedUpload{}, nil, err
	} else if exists {
		return storedUpload{}, nil, fmt.Errorf("duplicate media already exists in this event")
	}

	store, err := service.storageManager.ActiveStore()
	if err != nil {
		return storedUpload{}, nil, err
	}

	contentType := buffered.ContentType
	objectKey := mediaObjectKey(eventID, buffered.ContentHash, upload.Header.Filename, contentType)
	result := storedUpload{}
	thumbnailKey := ""

	if _, err := buffered.File.Seek(0, io.SeekStart); err != nil {
		return storedUpload{}, nil, fmt.Errorf("prepare upload file: %w", err)
	}
	stored, err := store.Put(ctx, storage.Object{
		Key:         objectKey,
		Content:     buffered.File,
		ContentType: contentType,
		Size:        buffered.SizeBytes,
	})
	if err != nil {
		return storedUpload{}, nil, err
	}

	var storedThumbnail storage.StoredObject
	if buffered.IsImage {
		if _, err := buffered.File.Seek(0, io.SeekStart); err != nil {
			_ = store.Delete(ctx, stored.Key)
			return storedUpload{}, nil, fmt.Errorf("prepare thumbnail source: %w", err)
		}
		content, err := io.ReadAll(buffered.File)
		if err != nil {
			_ = store.Delete(ctx, stored.Key)
			return storedUpload{}, nil, fmt.Errorf("read image for thumbnail: %w", err)
		}
		metadata := appimage.ExtractMetadataBytes(content)
		if raw, err := json.Marshal(metadata); err == nil && string(raw) != "{}" {
			result.Exif = raw
		}
		if metadata.TakenAt != "" {
			if parsed, err := time.Parse(time.RFC3339, metadata.TakenAt); err == nil {
				result.TakenAt = &parsed
			}
		}
		thumbnailKey = thumbnailObjectKey(eventID, buffered.ContentHash)
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
	}

	result.PolicyID = stored.PolicyID
	result.ObjectKey = stored.Key
	result.ThumbnailKey = storedThumbnail.Key
	result.ContentHash = buffered.ContentHash
	result.ContentType = contentType
	result.SizeBytes = buffered.SizeBytes
	return result, store, nil
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
	var count int64
	if err := service.db.WithContext(ctx).Model(&appdb.Photo{}).
		Where("event_id = ? AND content_hash = ?", eventID, contentHash).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check duplicate photo: %w", err)
	}
	if count > 0 {
		return true, nil
	}
	if err := service.db.WithContext(ctx).Model(&appdb.Submission{}).
		Where("event_id = ? AND content_hash = ? AND status = ?", eventID, contentHash, string(SubmissionPending)).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check duplicate submission: %w", err)
	}
	return count > 0, nil
}

func (service *Service) photoByID(ctx context.Context, photoID int64) (gallery.Photo, error) {
	var record appdb.Photo
	if err := service.db.WithContext(ctx).Where("id = ?", photoID).Take(&record).Error; err != nil {
		return gallery.Photo{}, err
	}
	photo := scanPhoto(record, service.storageManager)

	photos := []gallery.Photo{photo}
	if err := attachPhotoTags(ctx, service.db, photos); err != nil {
		return gallery.Photo{}, err
	}
	return photos[0], nil
}

func (service *Service) Get(ctx context.Context, id int64) (Submission, bool, error) {
	var record appdb.Submission
	err := service.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Submission{}, false, nil
	}
	if err != nil {
		return Submission{}, false, err
	}

	return service.submissionFromRecord(record), true, nil
}

func (service *Service) ListPending(ctx context.Context) ([]Submission, error) {
	return service.ListPendingForEvent(ctx, 0)
}

func (service *Service) ListPendingForEvent(ctx context.Context, eventID int64) ([]Submission, error) {
	if service.db == nil {
		return []Submission{}, nil
	}

	query := service.db.WithContext(ctx).Where("status = ?", string(SubmissionPending))
	if eventID > 0 {
		query = query.Where("event_id = ?", eventID)
	}

	var records []appdb.Submission
	if err := query.Order("created_at DESC").Order("id DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list pending submissions: %w", err)
	}

	submissions := make([]Submission, 0, len(records))
	for _, record := range records {
		submissions = append(submissions, service.submissionFromRecord(record))
	}

	return submissions, nil
}

func (service *Service) ApproveBatch(ctx context.Context, ids []int64, visibility string) (BatchResponse, error) {
	visibility = visibilityOrDefault(visibility)
	processed := 0
	for _, id := range ids {
		ok, err := service.approveOne(ctx, id, visibility)
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

func (service *Service) applySubmissionAccess(ctx context.Context, eventID int64, upload FileUpload) (FileUpload, error) {
	if strings.TrimSpace(upload.SubmissionToken) == "" {
		return upload, fmt.Errorf("submission link is required")
	}
	link, ok, err := service.consumeSubmissionLink(ctx, eventID, upload.SubmissionToken)
	if err != nil {
		return upload, err
	}
	if !ok {
		return upload, fmt.Errorf("invalid or expired submission link")
	}
	if strings.TrimSpace(link.PhotographerName) != "" {
		upload.PhotographerName = link.PhotographerName
	}
	return upload, nil
}

type consumedSubmissionLink struct {
	PhotographerName string
}

func (service *Service) consumeSubmissionLink(ctx context.Context, eventID int64, token string) (consumedSubmissionLink, bool, error) {
	tokenHash := auth.TokenHash(token)
	var link consumedSubmissionLink
	var consumed bool
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record appdb.SubmissionLink
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("event_id = ? AND token_hash = ?", eventID, tokenHash).
			Take(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("load submission link: %w", err)
		}
		now := time.Now()
		if record.RevokedAt != nil || (record.ExpiresAt != nil && now.After(*record.ExpiresAt)) || (record.MaxUses > 0 && record.UseCount >= record.MaxUses) {
			return nil
		}
		if err := tx.Model(&appdb.SubmissionLink{}).Where("id = ?", record.ID).Update("use_count", gorm.Expr("use_count + 1")).Error; err != nil {
			return fmt.Errorf("consume submission link: %w", err)
		}
		link.PhotographerName = stringValue(record.PhotographerName)
		consumed = true
		return nil
	})
	if err != nil {
		return link, false, err
	}
	return link, consumed, nil
}

func (service *Service) verifyEventAllowsSubmission(ctx context.Context, eventID int64) error {
	var event appdb.Event
	err := service.db.WithContext(ctx).Select("submission_enabled").Where("id = ?", eventID).Take(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("event not found")
	}
	if err != nil {
		return fmt.Errorf("load event submission settings: %w", err)
	}
	if !event.SubmissionEnabled {
		return fmt.Errorf("submissions are closed")
	}
	return nil
}

func (service *Service) approveOne(ctx context.Context, id int64, visibility string) (bool, error) {
	submission, found, err := service.Get(ctx, id)
	if err != nil {
		return false, err
	}
	if !found || submission.Status != SubmissionPending {
		return false, nil
	}
	var submissionRecord appdb.Submission
	if err := service.db.WithContext(ctx).Select("exif").Where("id = ?", id).Take(&submissionRecord).Error; err != nil {
		return false, fmt.Errorf("load submission metadata: %w", err)
	}

	if err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := appdb.Photo{
			EventID:          submission.EventID,
			StoragePolicyID:  submission.StoragePolicyID,
			ObjectKey:        submission.ObjectKey,
			ThumbnailKey:     stringPtr(submission.ThumbnailKey),
			ContentHash:      submission.ContentHash,
			ContentType:      submission.ContentType,
			SizeBytes:        submission.SizeBytes,
			PhotographerName: stringPtr(submission.PhotographerName),
			Visibility:       visibility,
			Exif:             submissionRecord.Exif,
			TakenAt:          submission.TakenAt,
			SortAt:           time.Now(),
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("create photo from submission: %w", err)
		}
		if err := upsertPhotoTags(ctx, tx, record.ID, submission.Tags); err != nil {
			return err
		}
		if err := tx.Where("id = ?", id).Delete(&appdb.Submission{}).Error; err != nil {
			return fmt.Errorf("remove approved submission: %w", err)
		}
		return nil
	}); err != nil {
		return false, err
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

	result := service.db.WithContext(ctx).Where("id = ?", id).Delete(&appdb.Submission{})
	if result.Error != nil {
		return false, fmt.Errorf("delete submission: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (service *Service) submissionFromRecord(record appdb.Submission) Submission {
	submission := Submission{
		ID:               record.ID,
		EventID:          record.EventID,
		StoragePolicyID:  record.StoragePolicyID,
		ObjectKey:        record.ObjectKey,
		ThumbnailKey:     stringValue(record.ThumbnailKey),
		ContentHash:      record.ContentHash,
		ContentType:      record.ContentType,
		SizeBytes:        record.SizeBytes,
		PhotographerName: stringValue(record.PhotographerName),
		Status:           SubmissionStatus(record.Status),
		Exif:             map[string]any{},
		TakenAt:          record.TakenAt,
		CreatedAt:        record.CreatedAt,
	}
	if store, err := service.storageManager.StoreForPolicy(submission.StoragePolicyID); err == nil {
		submission.URL = store.PublicURL(submission.ObjectKey)
		if submission.ThumbnailKey != "" {
			submission.ThumbnailURL = store.PublicURL(submission.ThumbnailKey)
		}
	} else {
		submission.URL = storage.MediaURL(submission.StoragePolicyID, submission.ObjectKey)
	}
	if len(record.Tags) > 0 {
		_ = json.Unmarshal(record.Tags, &submission.Tags)
	}
	if len(record.Exif) > 0 {
		_ = json.Unmarshal(record.Exif, &submission.Exif)
	}

	return submission
}

func upsertPhotoTags(ctx context.Context, tx *gorm.DB, photoID int64, tags []string) error {
	for _, tag := range normalizeTags(tags) {
		tagRecord := appdb.Tag{Name: tag}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoNothing: true,
		}).Create(&tagRecord).Error; err != nil {
			return fmt.Errorf("upsert tag: %w", err)
		}
		if tagRecord.ID == 0 {
			if err := tx.WithContext(ctx).Where("name = ?", tag).Take(&tagRecord).Error; err != nil {
				return fmt.Errorf("load tag id: %w", err)
			}
		}

		if err := tx.WithContext(ctx).Clauses(clause.Insert{Modifier: "IGNORE"}).Create(&appdb.PhotoTag{PhotoID: photoID, TagID: tagRecord.ID}).Error; err != nil {
			return fmt.Errorf("link photo tag: %w", err)
		}
	}

	return nil
}

func scanPhoto(record appdb.Photo, storageManager *storage.Manager) gallery.Photo {
	photo := gallery.Photo{
		ID:               record.ID,
		EventID:          record.EventID,
		StoragePolicyID:  record.StoragePolicyID,
		ObjectKey:        record.ObjectKey,
		ThumbnailKey:     stringValue(record.ThumbnailKey),
		ContentHash:      record.ContentHash,
		ContentType:      record.ContentType,
		SizeBytes:        record.SizeBytes,
		LikeCount:        record.LikeCount,
		PhotographerName: stringValue(record.PhotographerName),
		Visibility:       gallery.Visibility(record.Visibility),
		AccessGranted:    true,
		Tags:             []gallery.Tag{},
		Exif:             map[string]any{},
		TakenAt:          record.TakenAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
	if len(record.Exif) > 0 {
		_ = json.Unmarshal(record.Exif, &photo.Exif)
	}

	if store, err := storageManager.StoreForPolicy(photo.StoragePolicyID); err == nil {
		if photo.Visibility == gallery.VisibilityPublic {
			photo.URL = store.PublicURL(photo.ObjectKey)
		} else {
			photo.URL = mediaPhotoURL(photo.ID, "original")
		}
		if photo.ThumbnailKey != "" {
			if photo.Visibility == gallery.VisibilityPublic {
				photo.ThumbnailURL = store.PublicURL(photo.ThumbnailKey)
			} else {
				photo.ThumbnailURL = mediaPhotoURL(photo.ID, "thumbnail")
			}
		}
	} else {
		photo.URL = mediaPhotoURL(photo.ID, "original")
	}

	return photo
}

func mediaPhotoURL(photoID int64, variant string) string {
	return fmt.Sprintf("/media/photos/%d/%s", photoID, variant)
}

func attachPhotoTags(ctx context.Context, dbConn *gorm.DB, photos []gallery.Photo) error {
	if len(photos) == 0 {
		return nil
	}

	ids := make([]int64, len(photos))
	indexByID := map[int64]int{}
	for index, photo := range photos {
		ids[index] = photo.ID
		indexByID[photo.ID] = index
	}

	type tagRow struct {
		PhotoID   int64
		ID        int64
		Name      string
		CreatedAt time.Time
	}
	var rows []tagRow
	if err := dbConn.WithContext(ctx).
		Table("photo_tags").
		Select("photo_tags.photo_id, tags.id, tags.name, tags.created_at").
		Joins("INNER JOIN tags ON tags.id = photo_tags.tag_id").
		Where("photo_tags.photo_id IN ?", ids).
		Order("tags.name").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load photo tags: %w", err)
	}

	for _, row := range rows {
		if index, ok := indexByID[row.PhotoID]; ok {
			tag := gallery.Tag{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt}
			photos[index].Tags = append(photos[index].Tags, tag)
		}
	}

	return nil
}

func bufferUploadToTemp(source io.Reader, maxSizeBytes int64) (bufferedUpload, error) {
	tempFile, err := os.CreateTemp("", "fluffcatch-upload-*")
	if err != nil {
		return bufferedUpload{}, fmt.Errorf("create upload buffer: %w", err)
	}

	cleanup := func() {
		name := tempFile.Name()
		_ = tempFile.Close()
		_ = os.Remove(name)
	}

	hasher := sha256.New()
	head := make([]byte, 0, 512)
	limited := io.LimitReader(source, maxSizeBytes+1)
	written, err := copyWithHead(tempFile, hasher, limited, &head, 512)
	if err != nil {
		cleanup()
		return bufferedUpload{}, fmt.Errorf("read upload file: %w", err)
	}
	if written == 0 {
		cleanup()
		return bufferedUpload{}, fmt.Errorf("empty file is not allowed")
	}
	if written > maxSizeBytes {
		cleanup()
		return bufferedUpload{}, fmt.Errorf("file exceeds maximum upload size of %d MB", maxSizeBytes/(1024*1024))
	}

	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return bufferedUpload{}, fmt.Errorf("prepare upload buffer: %w", err)
	}

	contentType := http.DetectContentType(head)
	return bufferedUpload{
		File:        tempFile,
		SizeBytes:   written,
		ContentHash: hex.EncodeToString(hasher.Sum(nil)),
		ContentType: contentType,
		IsImage:     isAllowedImageContentType(contentType),
	}, nil
}

func copyWithHead(dst io.Writer, hasher hash.Hash, src io.Reader, head *[]byte, headLimit int) (int64, error) {
	buffer := make([]byte, 128*1024)
	var written int64
	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			if len(*head) < headLimit {
				need := min(headLimit-len(*head), len(chunk))
				*head = append(*head, chunk[:need]...)
			}
			if _, err := hasher.Write(chunk); err != nil {
				return written, err
			}
			if _, err := dst.Write(chunk); err != nil {
				return written, err
			}
			written += int64(n)
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

func mediaObjectKey(eventID int64, contentHash string, filename string, contentType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = extensionForContentType(contentType)
	}
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("events/%d/media/%s%s", eventID, contentHash, ext)
}

func thumbnailObjectKey(eventID int64, contentHash string) string {
	return fmt.Sprintf("events/%d/thumbnails/%s.jpg", eventID, contentHash)
}

func isAllowedImageContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func isAllowedVideoContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "video/mp4", "video/webm", "video/ogg", "video/quicktime":
		return true
	default:
		return false
	}
}

func isAllowedMediaContentType(contentType string) bool {
	return isAllowedImageContentType(contentType) || isAllowedVideoContentType(contentType)
}

func limitForContentType(contentType string, limits uploadLimits) int64 {
	if isAllowedVideoContentType(contentType) {
		return limits.MaxVideoBytes
	}
	return limits.MaxImageBytes
}

func mediaKind(contentType string) string {
	if isAllowedVideoContentType(contentType) {
		return "video"
	}
	return "image"
}

func extensionForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/ogg":
		return ".ogv"
	case "video/quicktime":
		return ".mov"
	default:
		return ""
	}
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

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
