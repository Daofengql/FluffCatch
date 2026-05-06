package gallery

import (
	"context"
	"database/sql"
	"fmt"
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
	if service.db == nil {
		return []Photo{}, nil
	}

	query := `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			original_filename, COALESCE(photographer_name, ''), visibility, taken_at, created_at, updated_at
		FROM photos
		WHERE event_id = ?
	`
	args := []any{eventID}
	if !admin {
		query += " AND visibility = 'public'"
	}
	query += " ORDER BY COALESCE(taken_at, created_at) DESC, id DESC"

	rows, err := service.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list photos: %w", err)
	}
	defer rows.Close()

	photos := []Photo{}
	for rows.Next() {
		photo, err := service.scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate photos: %w", err)
	}

	if err := service.attachTags(ctx, photos); err != nil {
		return nil, err
	}

	return photos, nil
}

func (service *Service) UpdatePhoto(ctx context.Context, photoID int64, req UpdatePhotoRequest) (Photo, error) {
	if req.Visibility == "" {
		req.Visibility = VisibilityPublic
	}
	switch req.Visibility {
	case VisibilityPublic, VisibilityProtected, VisibilityPrivate:
	default:
		return Photo{}, fmt.Errorf("unsupported visibility %q", req.Visibility)
	}

	var passwordHash string
	var err error
	if req.AccessPassword != "" {
		passwordHash, err = auth.HashPassword(req.AccessPassword)
		if err != nil {
			return Photo{}, err
		}
	}

	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return Photo{}, fmt.Errorf("begin update photo: %w", err)
	}
	defer tx.Rollback()

	if req.AccessPassword == "" {
		_, err = tx.ExecContext(ctx, "UPDATE photos SET visibility = ? WHERE id = ?", req.Visibility, photoID)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE photos SET visibility = ?, access_password_hash = ? WHERE id = ?", req.Visibility, passwordHash, photoID)
	}
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

func (service *Service) Get(ctx context.Context, photoID int64) (Photo, bool, error) {
	row := service.db.QueryRowContext(ctx, `
		SELECT id, event_id, storage_policy_id, object_key, COALESCE(thumbnail_key, ''),
			original_filename, COALESCE(photographer_name, ''), visibility, taken_at, created_at, updated_at
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
		&photo.OriginalFilename,
		&photo.PhotographerName,
		&visibility,
		&takenAt,
		&photo.CreatedAt,
		&photo.UpdatedAt,
	); err != nil {
		return Photo{}, err
	}

	photo.Visibility = Visibility(visibility)
	photo.ThumbnailKey = thumbnailKey
	if takenAt.Valid {
		photo.TakenAt = &takenAt.Time
	}

	if store, err := service.storageManager.StoreForPolicy(photo.StoragePolicyID); err == nil {
		photo.URL = store.PublicURL(photo.ObjectKey)
		if photo.ThumbnailKey != "" {
			photo.ThumbnailURL = store.PublicURL(photo.ThumbnailKey)
		}
	} else {
		photo.URL = storage.MediaURL(photo.StoragePolicyID, photo.ObjectKey)
	}

	return photo, nil
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
