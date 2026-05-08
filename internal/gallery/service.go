package gallery

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/storage"
)

type Service struct {
	db             *sql.DB
	storageManager *storage.Manager
}

func NewService(dbConn *sql.DB, storageManager *storage.Manager) *Service {
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
	if service.db == nil {
		return Page{Items: []Photo{}, Page: 1, PageSize: pageSize}, nil
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if !admin {
		public, err := service.eventIsPublic(ctx, eventID)
		if err != nil {
			return Page{}, err
		}
		if !public {
			return Page{Items: []Photo{}, Page: page, PageSize: pageSize}, nil
		}
	}

	where := "WHERE event_id = ?"
	args := []any{eventID}
	if !admin {
		where += " AND visibility IN ('public', 'private')"
	}

	var total int64
	if err := service.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM photos "+where, args...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count photos: %w", err)
	}

	query := `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			content_hash, content_type, size_bytes, like_count, COALESCE(photographer_name, ''), visibility, taken_at, created_at, updated_at
		FROM photos
		` + where + `
		ORDER BY sort_at DESC, id DESC
		LIMIT ? OFFSET ?
	`
	offset := (page - 1) * pageSize
	queryArgs := append(args, pageSize, offset)

	rows, err := service.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return Page{}, fmt.Errorf("list photos: %w", err)
	}
	defer rows.Close()

	photos := []Photo{}
	for rows.Next() {
		photo, err := service.scanPhoto(rows)
		if err != nil {
			return Page{}, err
		}
		if !admin && photo.Visibility == VisibilityPrivate && privateAccess {
			photo.AccessGranted = true
		}
		if admin || photo.Visibility == VisibilityPublic {
			photo.AccessGranted = true
		}
		service.applyPhotoURLs(&photo)
		photos = append(photos, photo)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate photos: %w", err)
	}

	if err := service.attachTags(ctx, photos); err != nil {
		return Page{}, err
	}
	if err := service.attachLiked(ctx, photos, fingerprintHash); err != nil {
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

	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return Photo{}, fmt.Errorf("begin update photo: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "UPDATE photos SET visibility = ?, photographer_name = NULLIF(?, '') WHERE id = ?", req.Visibility, strings.TrimSpace(req.PhotographerName), photoID)
	if err != nil {
		return Photo{}, fmt.Errorf("update photo: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM photo_tags WHERE photo_id = ?", photoID); err != nil {
		return Photo{}, fmt.Errorf("clear photo tags: %w", err)
	}
	if err := upsertPhotoTags(ctx, tx, photoID, req.Tags); err != nil {
		return Photo{}, err
	}

	if err := tx.Commit(); err != nil {
		return Photo{}, fmt.Errorf("commit update photo: %w", err)
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

	placeholders := make([]string, len(req.PhotoIDs))
	idArgs := make([]any, len(req.PhotoIDs))
	for i, id := range req.PhotoIDs {
		placeholders[i] = "?"
		idArgs[i] = id
	}

	countArgs := make([]any, 0, len(req.PhotoIDs)+1)
	countArgs = append(countArgs, req.Visibility)
	countArgs = append(countArgs, idArgs...)
	var matched int
	if err := service.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM photos WHERE visibility <> ? AND id IN ("+strings.Join(placeholders, ",")+")", countArgs...).Scan(&matched); err != nil {
		return 0, fmt.Errorf("count photos for batch update: %w", err)
	}
	if matched == 0 {
		return 0, nil
	}

	if _, err := service.db.ExecContext(ctx, "UPDATE photos SET visibility = ? WHERE id IN ("+strings.Join(placeholders, ",")+")", countArgs...); err != nil {
		return 0, fmt.Errorf("batch update photos: %w", err)
	}
	return matched, nil
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

	result, err := service.db.ExecContext(ctx, "DELETE FROM photos WHERE id = ?", photoID)
	if err != nil {
		return false, nil, fmt.Errorf("delete photo: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return true, objects, nil
	}
	return affected > 0, objects, nil
}

func (service *Service) Get(ctx context.Context, photoID int64) (Photo, bool, error) {
	row := service.db.QueryRowContext(ctx, `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			content_hash, content_type, size_bytes, like_count, COALESCE(photographer_name, ''), visibility, taken_at, created_at, updated_at
		FROM photos
		WHERE id = ?
		LIMIT 1
	`, photoID)
	photo, err := service.scanPhoto(row)
	if err == sql.ErrNoRows {
		return Photo{}, false, nil
	}
	if err != nil {
		return Photo{}, false, err
	}
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

	var visibility string
	var likeCount int64
	var isPublic bool
	err := service.db.QueryRowContext(ctx, `
		SELECT photos.visibility, photos.like_count, events.is_public
		FROM photos
		INNER JOIN events ON events.id = photos.event_id
		WHERE photos.id = ?
		LIMIT 1
	`, photoID).Scan(&visibility, &likeCount, &isPublic)
	if err == sql.ErrNoRows {
		return LikeResult{}, fmt.Errorf("photo not found")
	}
	if err != nil {
		return LikeResult{}, fmt.Errorf("load photo for like: %w", err)
	}
	if !isPublic || visibility != string(VisibilityPublic) {
		return LikeResult{}, fmt.Errorf("photo is not public")
	}

	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return LikeResult{}, fmt.Errorf("begin like photo: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO photo_likes (photo_id, fingerprint_hash)
		VALUES (?, ?)
	`, photoID, fingerprintHash)
	if err != nil {
		return LikeResult{}, fmt.Errorf("record photo like: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return LikeResult{}, fmt.Errorf("read like result: %w", err)
	}
	justLiked := affected > 0
	if justLiked {
		if _, err := tx.ExecContext(ctx, "UPDATE photos SET like_count = like_count + 1 WHERE id = ?", photoID); err != nil {
			return LikeResult{}, fmt.Errorf("increment photo like count: %w", err)
		}
	}

	if err := tx.QueryRowContext(ctx, "SELECT like_count FROM photos WHERE id = ? LIMIT 1", photoID).Scan(&likeCount); err != nil {
		return LikeResult{}, fmt.Errorf("load photo like count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return LikeResult{}, fmt.Errorf("commit photo like: %w", err)
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
	var public bool
	err := service.db.QueryRowContext(ctx, "SELECT is_public FROM events WHERE id = ? LIMIT 1", eventID).Scan(&public)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load event visibility: %w", err)
	}
	return public, nil
}

func (service *Service) verifyEventPrivatePassword(ctx context.Context, eventID int64, password string) (bool, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return false, nil
	}

	var passwordHash sql.NullString
	err := service.db.QueryRowContext(ctx, "SELECT private_password_hash FROM events WHERE id = ? LIMIT 1", eventID).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load event private password: %w", err)
	}
	if !passwordHash.Valid || passwordHash.String == "" {
		return false, nil
	}

	ok, err := auth.VerifyPassword(password, passwordHash.String)
	if err != nil {
		return false, nil
	}
	return ok, nil
}

type photoScanner interface {
	Scan(dest ...any) error
}

func (service *Service) scanPhoto(scanner photoScanner) (Photo, error) {
	var photo Photo
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
		return Photo{}, err
	}

	photo.Tags = []Tag{}
	photo.Visibility = Visibility(visibility)
	photo.ThumbnailKey = thumbnailKey
	if takenAt.Valid {
		photo.TakenAt = &takenAt.Time
	}

	service.applyPhotoURLs(&photo)

	return photo, nil
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

	ids := make([]string, len(photos))
	args := make([]any, 0, len(photos)+1)
	indexByID := map[int64]int{}
	for index, photo := range photos {
		ids[index] = "?"
		args = append(args, photo.ID)
		indexByID[photo.ID] = index
	}
	args = append(args, fingerprintHash)

	rows, err := service.db.QueryContext(ctx, `
		SELECT photo_id
		FROM photo_likes
		WHERE photo_id IN (`+strings.Join(ids, ",")+`)
			AND fingerprint_hash = ?
	`, args...)
	if err != nil {
		return fmt.Errorf("load liked photos: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var photoID int64
		if err := rows.Scan(&photoID); err != nil {
			return fmt.Errorf("scan liked photo: %w", err)
		}
		if index, ok := indexByID[photoID]; ok {
			photos[index].Liked = true
		}
	}

	return rows.Err()
}

func (service *Service) attachTags(ctx context.Context, photos []Photo) error {
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

	rows, err := service.db.QueryContext(ctx, `
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
		var tag Tag
		if err := rows.Scan(&photoID, &tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return fmt.Errorf("scan photo tag: %w", err)
		}
		if index, ok := indexByID[photoID]; ok {
			photos[index].Tags = append(photos[index].Tags, tag)
		}
	}

	return rows.Err()
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
