package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	appdb "fluffcatch/internal/db"
	appimage "fluffcatch/internal/image"
	"fluffcatch/internal/storage"

	"gorm.io/gorm"
)

type EXIFBackfillResult struct {
	Scanned        int
	Updated        int
	Skipped        int
	Failed         int
	BytesRead      int64
	PhotoRows      int
	SubmissionRows int
}

type exifBackfillRow struct {
	ID              int64
	Kind            string
	StoragePolicyID string
	ObjectKey       string
	ContentType     string
	Exif            []byte
	TakenAt         *time.Time
}

func BackfillEXIF(ctx context.Context, dbConn *gorm.DB, storageManager *storage.Manager) (EXIFBackfillResult, error) {
	if dbConn == nil {
		return EXIFBackfillResult{}, fmt.Errorf("database is required")
	}
	if storageManager == nil {
		return EXIFBackfillResult{}, fmt.Errorf("storage manager is required")
	}

	rows, err := loadEXIFBackfillRows(ctx, dbConn)
	if err != nil {
		return EXIFBackfillResult{}, err
	}

	result := EXIFBackfillResult{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.Scanned++
		if row.Kind == "photo" {
			result.PhotoRows++
		} else {
			result.SubmissionRows++
		}

		metadata, size, err := readStoredImageMetadata(ctx, storageManager, row)
		result.BytesRead += size
		if err != nil {
			result.Failed++
			slog.Warn("exif backfill failed to read image", "kind", row.Kind, "id", row.ID, "policy", row.StoragePolicyID, "key", row.ObjectKey, "error", err)
			continue
		}
		if isEmptyImageMetadata(metadata) {
			result.Skipped++
			continue
		}

		raw, err := json.Marshal(metadata)
		if err != nil {
			result.Failed++
			slog.Warn("exif backfill failed to encode metadata", "kind", row.Kind, "id", row.ID, "error", err)
			continue
		}
		takenAt := row.TakenAt
		if metadata.TakenAt != "" {
			if parsed, err := time.Parse(time.RFC3339, metadata.TakenAt); err == nil {
				takenAt = &parsed
			}
		}
		if string(raw) == string(row.Exif) && equalTimePtr(takenAt, row.TakenAt) {
			result.Skipped++
			continue
		}
		if err := updateEXIFBackfillRow(ctx, dbConn, row, raw, takenAt); err != nil {
			result.Failed++
			slog.Warn("exif backfill failed to update database", "kind", row.Kind, "id", row.ID, "error", err)
			continue
		}
		result.Updated++
	}
	return result, nil
}

func loadEXIFBackfillRows(ctx context.Context, dbConn *gorm.DB) ([]exifBackfillRow, error) {
	rows := []exifBackfillRow{}
	photoQuery := dbConn.WithContext(ctx).
		Table("photos").
		Select("id, 'photo' AS kind, storage_policy_id, object_key, content_type, exif, taken_at").
		Where("content_type LIKE ?", "image/%").
		Order("id ASC")
	if err := photoQuery.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load photos for exif backfill: %w", err)
	}

	submissions := []exifBackfillRow{}
	submissionQuery := dbConn.WithContext(ctx).
		Table("submissions").
		Select("id, 'submission' AS kind, storage_policy_id, object_key, content_type, exif, taken_at").
		Where("content_type LIKE ?", "image/%").
		Order("id ASC")
	if err := submissionQuery.Scan(&submissions).Error; err != nil {
		return nil, fmt.Errorf("load submissions for exif backfill: %w", err)
	}
	rows = append(rows, submissions...)
	return rows, nil
}

func readStoredImageMetadata(ctx context.Context, storageManager *storage.Manager, row exifBackfillRow) (appimage.Metadata, int64, error) {
	store, err := storageManager.StoreForPolicy(row.StoragePolicyID)
	if err != nil {
		return appimage.Metadata{}, 0, err
	}
	object, err := store.Get(ctx, row.ObjectKey)
	if err != nil {
		return appimage.Metadata{}, 0, err
	}
	defer object.Content.Close()

	content, err := io.ReadAll(object.Content)
	if err != nil {
		return appimage.Metadata{}, 0, fmt.Errorf("read object: %w", err)
	}
	return appimage.ExtractMetadataBytes(content), int64(len(content)), nil
}

func updateEXIFBackfillRow(ctx context.Context, dbConn *gorm.DB, row exifBackfillRow, exif []byte, takenAt *time.Time) error {
	updates := map[string]any{"exif": exif}
	if takenAt != nil {
		updates["taken_at"] = takenAt
	}
	switch row.Kind {
	case "photo":
		return dbConn.WithContext(ctx).Model(&appdb.Photo{}).Where("id = ?", row.ID).Updates(updates).Error
	case "submission":
		return dbConn.WithContext(ctx).Model(&appdb.Submission{}).Where("id = ?", row.ID).Updates(updates).Error
	default:
		return fmt.Errorf("unsupported row kind %q", row.Kind)
	}
}

func isEmptyImageMetadata(metadata appimage.Metadata) bool {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return true
	}
	return string(raw) == "{}"
}

func equalTimePtr(left *time.Time, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func IsNoBackfillWork(result EXIFBackfillResult) bool {
	return result.Scanned == 0 && result.Updated == 0 && result.Failed == 0
}

func IsContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
