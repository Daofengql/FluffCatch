package gallery

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"fluffcatch/internal/auth"
	appdb "fluffcatch/internal/db"
	"fluffcatch/internal/storage"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db             *gorm.DB
	storageManager *storage.Manager
}

func NewService(dbConn *gorm.DB, storageManager *storage.Manager) *Service {
	return &Service{
		db:             dbConn,
		storageManager: storageManager,
	}
}

func (service *Service) ListForEvent(ctx context.Context, eventID int64, admin bool, password string) ([]Photo, error) {
	page, err := service.ListForEventPage(ctx, eventID, admin, password, 1, 500)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (service *Service) ListForEventPage(ctx context.Context, eventID int64, admin bool, password string, page int, pageSize int) (Page, error) {
	return service.ListForEventPageWithFingerprint(ctx, eventID, admin, password, "", page, pageSize)
}

func (service *Service) ListForEventPageWithFingerprint(ctx context.Context, eventID int64, admin bool, password string, fingerprintHash string, page int, pageSize int) (Page, error) {
	privateAccess := false
	if password != "" {
		ok, err := service.verifyEventPrivatePassword(ctx, eventID, password)
		if err != nil {
			return Page{}, err
		}
		privateAccess = ok
	}
	return service.ListForEventPageWithAccess(ctx, eventID, admin, privateAccess, fingerprintHash, page, pageSize)
}

func (service *Service) ListForEventPageWithAccess(ctx context.Context, eventID int64, admin bool, privateAccess bool, fingerprintHash string, page int, pageSize int) (Page, error) {
	return service.ListForEventPageWithOptions(ctx, eventID, ListOptions{
		Admin:           admin,
		PrivateAccess:   privateAccess,
		FingerprintHash: fingerprintHash,
		Page:            page,
		PageSize:        pageSize,
	})
}

func (service *Service) ListForEventPageWithOptions(ctx context.Context, eventID int64, options ListOptions) (Page, error) {
	if service.db == nil {
		return Page{Items: []Photo{}, Page: 1, PageSize: options.PageSize}, nil
	}
	page := options.Page
	if page < 1 {
		page = 1
	}
	pageSize := options.PageSize
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if !options.Admin {
		public, err := service.eventIsPublic(ctx, eventID)
		if err != nil {
			return Page{}, err
		}
		if !public {
			return Page{Items: []Photo{}, Page: page, PageSize: pageSize}, nil
		}
	}

	var total int64
	baseQuery := service.db.WithContext(ctx).Model(&appdb.Photo{}).Where("event_id = ?", eventID)
	switch options.Visibility {
	case VisibilityPublic, VisibilityPrivate:
		baseQuery = baseQuery.Where("visibility = ?", string(options.Visibility))
	case "":
	default:
		return Page{}, fmt.Errorf("unsupported visibility %q", options.Visibility)
	}
	if !options.Admin {
		baseQuery = baseQuery.Where("visibility IN ?", []string{string(VisibilityPublic), string(VisibilityPrivate)})
	}
	if err := baseQuery.Count(&total).Error; err != nil {
		return Page{}, fmt.Errorf("count photos: %w", err)
	}

	offset := (page - 1) * pageSize
	var records []appdb.Photo
	if err := baseQuery.Order("sort_at DESC").Order("id DESC").Limit(pageSize).Offset(offset).Find(&records).Error; err != nil {
		return Page{}, fmt.Errorf("list photos: %w", err)
	}

	photos := make([]Photo, 0, len(records))
	for _, record := range records {
		photo := service.photoFromRecord(record)
		if !options.Admin && photo.Visibility == VisibilityPrivate && options.PrivateAccess {
			photo.AccessGranted = true
		}
		if options.Admin || photo.Visibility == VisibilityPublic {
			photo.AccessGranted = true
		}
		service.applyPhotoURLs(&photo)
		photos = append(photos, photo)
	}

	if err := service.attachTags(ctx, photos); err != nil {
		return Page{}, err
	}
	if err := service.attachLiked(ctx, photos, options.FingerprintHash); err != nil {
		return Page{}, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}
	return Page{Items: photos, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
}

func (service *Service) UpdatePhoto(ctx context.Context, photoID int64, req UpdatePhotoRequest) (Photo, error) {
	if req.Visibility == "" {
		req.Visibility = VisibilityPublic
	}
	switch req.Visibility {
	case VisibilityPublic, VisibilityPrivate:
	default:
		return Photo{}, fmt.Errorf("unsupported visibility %q", req.Visibility)
	}

	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&appdb.Photo{}).Where("id = ?", photoID).Updates(map[string]any{
			"visibility":        req.Visibility,
			"photographer_name": stringPtr(req.PhotographerName),
		}).Error; err != nil {
			return fmt.Errorf("update photo: %w", err)
		}

		if err := tx.Where("photo_id = ?", photoID).Delete(&appdb.PhotoTag{}).Error; err != nil {
			return fmt.Errorf("clear photo tags: %w", err)
		}
		if err := upsertPhotoTags(ctx, tx, photoID, req.Tags); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Photo{}, err
	}

	photo, found, err := service.Get(ctx, photoID)
	if err != nil {
		return Photo{}, err
	}
	if !found {
		return Photo{}, fmt.Errorf("photo not found")
	}
	return photo, nil
}

func (service *Service) BatchUpdatePhotos(ctx context.Context, req BatchUpdatePhotosRequest) (int, error) {
	if req.Visibility == "" {
		req.Visibility = VisibilityPublic
	}
	switch req.Visibility {
	case VisibilityPublic, VisibilityPrivate:
	default:
		return 0, fmt.Errorf("unsupported visibility %q", req.Visibility)
	}
	if len(req.PhotoIDs) == 0 {
		return 0, nil
	}

	var matched int64
	if err := service.db.WithContext(ctx).Model(&appdb.Photo{}).Where("visibility <> ? AND id IN ?", req.Visibility, req.PhotoIDs).Count(&matched).Error; err != nil {
		return 0, fmt.Errorf("count photos for batch update: %w", err)
	}
	if matched == 0 {
		return 0, nil
	}

	if err := service.db.WithContext(ctx).Model(&appdb.Photo{}).Where("visibility <> ? AND id IN ?", req.Visibility, req.PhotoIDs).Update("visibility", req.Visibility).Error; err != nil {
		return 0, fmt.Errorf("batch update photos: %w", err)
	}
	return int(matched), nil
}

func (service *Service) DeletePhoto(ctx context.Context, photoID int64) (bool, []storage.StoredObject, error) {
	photo, found, err := service.Get(ctx, photoID)
	if err != nil {
		return false, nil, err
	}
	if !found {
		return false, nil, nil
	}

	objects := []storage.StoredObject{{PolicyID: photo.StoragePolicyID, Key: photo.ObjectKey}}
	if photo.ThumbnailKey != "" {
		objects = append(objects, storage.StoredObject{PolicyID: photo.StoragePolicyID, Key: photo.ThumbnailKey})
	}

	result := service.db.WithContext(ctx).Where("id = ?", photoID).Delete(&appdb.Photo{})
	if result.Error != nil {
		return false, nil, fmt.Errorf("delete photo: %w", result.Error)
	}
	return result.RowsAffected > 0, objects, nil
}

func (service *Service) Get(ctx context.Context, photoID int64) (Photo, bool, error) {
	var record appdb.Photo
	err := service.db.WithContext(ctx).Where("id = ?", photoID).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Photo{}, false, nil
	}
	if err != nil {
		return Photo{}, false, err
	}
	photo := service.photoFromRecord(record)
	photos := []Photo{photo}
	if err := service.attachTags(ctx, photos); err != nil {
		return Photo{}, false, err
	}
	return photos[0], true, nil
}

func (service *Service) Like(ctx context.Context, photoID int64, fingerprintHash string) (LikeResult, error) {
	if service.db == nil {
		return LikeResult{}, fmt.Errorf("database is required")
	}
	fingerprintHash = strings.TrimSpace(fingerprintHash)
	if fingerprintHash == "" {
		return LikeResult{}, fmt.Errorf("viewer fingerprint is required")
	}

	var photoRecord appdb.Photo
	err := service.db.WithContext(ctx).Where("id = ?", photoID).Take(&photoRecord).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LikeResult{}, fmt.Errorf("photo not found")
	}
	if err != nil {
		return LikeResult{}, fmt.Errorf("load photo for like: %w", err)
	}
	var event appdb.Event
	if err := service.db.WithContext(ctx).Select("is_public").Where("id = ?", photoRecord.EventID).Take(&event).Error; err != nil {
		return LikeResult{}, fmt.Errorf("load photo event for like: %w", err)
	}
	if !event.IsPublic || photoRecord.Visibility != string(VisibilityPublic) {
		return LikeResult{}, fmt.Errorf("photo is not public")
	}

	var likeCount int64
	justLiked := false
	if err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Insert{Modifier: "IGNORE"}).Create(&appdb.PhotoLike{PhotoID: photoID, FingerprintHash: fingerprintHash})
		if result.Error != nil {
			return fmt.Errorf("record photo like: %w", result.Error)
		}
		justLiked = result.RowsAffected > 0
		if justLiked {
			if err := tx.Model(&appdb.Photo{}).Where("id = ?", photoID).Update("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
				return fmt.Errorf("increment photo like count: %w", err)
			}
		}
		if err := tx.Model(&appdb.Photo{}).Select("like_count").Where("id = ?", photoID).Take(&likeCount).Error; err != nil {
			return fmt.Errorf("load photo like count: %w", err)
		}
		return nil
	}); err != nil {
		return LikeResult{}, err
	}

	return LikeResult{
		PhotoID:   photoID,
		LikeCount: likeCount,
		Liked:     true,
		JustLiked: justLiked,
	}, nil
}

func (service *Service) CanAccessPhoto(ctx context.Context, photoID int64, admin bool, password string) (Photo, bool, error) {
	privateAccess := false
	if password != "" {
		photo, found, err := service.Get(ctx, photoID)
		if err != nil || !found {
			return Photo{}, false, err
		}
		ok, err := service.verifyEventPrivatePassword(ctx, photo.EventID, password)
		if err != nil {
			return Photo{}, false, err
		}
		privateAccess = ok
	}
	return service.CanAccessPhotoWithAccess(ctx, photoID, admin, privateAccess)
}

func (service *Service) CanAccessPhotoWithAccess(ctx context.Context, photoID int64, admin bool, privateAccess bool) (Photo, bool, error) {
	photo, found, err := service.Get(ctx, photoID)
	if err != nil || !found {
		return Photo{}, false, err
	}
	if admin {
		photo.AccessGranted = true
		service.applyPhotoURLs(&photo)
		return photo, true, nil
	}
	public, err := service.eventIsPublic(ctx, photo.EventID)
	if err != nil {
		return Photo{}, false, err
	}
	if !public {
		return photo, false, nil
	}
	switch photo.Visibility {
	case VisibilityPublic:
		photo.AccessGranted = true
		service.applyPhotoURLs(&photo)
		return photo, true, nil
	case VisibilityPrivate:
		photo.AccessGranted = privateAccess
		service.applyPhotoURLs(&photo)
		return photo, privateAccess, nil
	default:
		return photo, false, nil
	}
}

func (service *Service) VerifyEventPrivatePassword(ctx context.Context, eventID int64, password string) (bool, error) {
	return service.verifyEventPrivatePassword(ctx, eventID, password)
}

func (service *Service) eventIsPublic(ctx context.Context, eventID int64) (bool, error) {
	var event appdb.Event
	err := service.db.WithContext(ctx).Select("is_public").Where("id = ?", eventID).Take(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load event visibility: %w", err)
	}
	return event.IsPublic, nil
}

func (service *Service) verifyEventPrivatePassword(ctx context.Context, eventID int64, password string) (bool, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return false, nil
	}

	var event appdb.Event
	err := service.db.WithContext(ctx).Select("private_password_hash").Where("id = ?", eventID).Take(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load event private password: %w", err)
	}
	if event.PrivatePasswordHash == nil || *event.PrivatePasswordHash == "" {
		return false, nil
	}

	ok, err := auth.VerifyPassword(password, *event.PrivatePasswordHash)
	if err != nil {
		return false, nil
	}
	return ok, nil
}

func (service *Service) photoFromRecord(record appdb.Photo) Photo {
	photo := Photo{
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
		Visibility:       Visibility(record.Visibility),
		Tags:             []Tag{},
		TakenAt:          record.TakenAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
	service.applyPhotoURLs(&photo)
	return photo
}

func (service *Service) applyPhotoURLs(photo *Photo) {
	if photo.Visibility == VisibilityPublic {
		if store, err := service.storageManager.StoreForPolicy(photo.StoragePolicyID); err == nil {
			photo.URL = store.PublicURL(photo.ObjectKey)
			if photo.ThumbnailKey != "" {
				photo.ThumbnailURL = store.PublicURL(photo.ThumbnailKey)
			}
			return
		}
	}

	photo.URL = mediaPhotoURL(photo.ID, "original")
	if photo.ThumbnailKey == "" {
		photo.ThumbnailURL = ""
		return
	}
	if photo.Visibility == VisibilityPrivate && !photo.AccessGranted {
		photo.ThumbnailURL = mediaPhotoURL(photo.ID, "blur")
		return
	}
	photo.ThumbnailURL = mediaPhotoURL(photo.ID, "thumbnail")
}

func mediaPhotoURL(photoID int64, variant string) string {
	return fmt.Sprintf("/media/photos/%d/%s", photoID, variant)
}

func (service *Service) attachLiked(ctx context.Context, photos []Photo, fingerprintHash string) error {
	fingerprintHash = strings.TrimSpace(fingerprintHash)
	if len(photos) == 0 || fingerprintHash == "" {
		return nil
	}

	ids := make([]int64, len(photos))
	indexByID := map[int64]int{}
	for index, photo := range photos {
		ids[index] = photo.ID
		indexByID[photo.ID] = index
	}

	var likes []appdb.PhotoLike
	if err := service.db.WithContext(ctx).
		Select("photo_id").
		Where("photo_id IN ? AND fingerprint_hash = ?", ids, fingerprintHash).
		Find(&likes).Error; err != nil {
		return fmt.Errorf("load liked photos: %w", err)
	}

	for _, like := range likes {
		if index, ok := indexByID[like.PhotoID]; ok {
			photos[index].Liked = true
		}
	}

	return nil
}

func (service *Service) attachTags(ctx context.Context, photos []Photo) error {
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
	if err := service.db.WithContext(ctx).
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
			tag := Tag{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt}
			photos[index].Tags = append(photos[index].Tags, tag)
		}
	}

	return nil
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
