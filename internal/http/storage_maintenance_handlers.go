package http

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"strings"

	appdb "fluffcatch/internal/db"
)

type storageScanItem struct {
	PolicyID string `json:"policyId"`
	Key      string `json:"key"`
	Size     int64  `json:"size,omitempty"`
}

func (server *Server) scanOrphanStorageObjects(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	used, err := server.usedStorageKeys(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load storage references")
		return
	}

	items := []storageScanItem{}
	var totalSize int64
	total := 0
	scannedPolicies := 0
	skippedPolicies := 0
	for _, config := range server.storageManager.Configs() {
		if config.Driver != "local" {
			skippedPolicies++
			continue
		}
		scannedPolicies++
		root := config.LocalPath
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			key := filepath.ToSlash(rel)
			if used[storageRefKey(config.PolicyID, key)] {
				return nil
			}
			info, err := entry.Info()
			if err == nil {
				totalSize += info.Size()
			}
			total++
			if len(items) < 200 {
				size := int64(0)
				if info != nil {
					size = info.Size()
				}
				items = append(items, storageScanItem{PolicyID: config.PolicyID, Key: key, Size: size})
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			writeError(w, stdhttp.StatusInternalServerError, fmt.Sprintf("failed to scan policy %s", config.PolicyID))
			return
		}
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"items":           items,
		"total":           total,
		"totalSizeBytes":  totalSize,
		"scannedPolicies": scannedPolicies,
		"skippedPolicies": skippedPolicies,
		"truncated":       total > len(items),
	})
}

func (server *Server) scanMissingThumbnails(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	type row struct {
		ID              int64  `gorm:"column:id" json:"id"`
		EventID         int64  `gorm:"column:event_id" json:"eventId"`
		Kind            string `gorm:"column:kind" json:"kind"`
		StoragePolicyID string `gorm:"column:storage_policy_id" json:"storagePolicyId"`
		ObjectKey       string `gorm:"column:object_key" json:"objectKey"`
		ContentType     string `gorm:"column:content_type" json:"contentType"`
	}
	items := []row{}
	photoQuery := server.db.WithContext(r.Context()).
		Table("photos").
		Select("'photo' AS kind, id, event_id, storage_policy_id, object_key, content_type").
		Where("content_type LIKE ? AND (thumbnail_key IS NULL OR thumbnail_key = '')", "image/%")
	submissionQuery := server.db.WithContext(r.Context()).
		Table("submissions").
		Select("'submission' AS kind, id, event_id, storage_policy_id, object_key, content_type").
		Where("content_type LIKE ? AND (thumbnail_key IS NULL OR thumbnail_key = '')", "image/%")
	if err := photoQuery.Scan(&items).Error; err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to scan photos")
		return
	}
	var submissionItems []row
	if err := submissionQuery.Scan(&submissionItems).Error; err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to scan submissions")
		return
	}
	items = append(items, submissionItems...)
	total := len(items)
	truncated := false
	if len(items) > 200 {
		items = items[:200]
		truncated = true
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"items": items, "total": total, "truncated": truncated})
}

func (server *Server) usedStorageKeys(ctx context.Context) (map[string]bool, error) {
	used := map[string]bool{}
	add := func(policyID string, key *string) {
		if strings.TrimSpace(policyID) == "" || key == nil || strings.TrimSpace(*key) == "" {
			return
		}
		used[storageRefKey(policyID, *key)] = true
	}

	var photos []appdb.Photo
	if err := server.db.WithContext(ctx).Select("storage_policy_id", "object_key", "thumbnail_key").Find(&photos).Error; err != nil {
		return nil, err
	}
	for _, photo := range photos {
		objectKey := photo.ObjectKey
		add(photo.StoragePolicyID, &objectKey)
		add(photo.StoragePolicyID, photo.ThumbnailKey)
	}

	var submissions []appdb.Submission
	if err := server.db.WithContext(ctx).Select("storage_policy_id", "object_key", "thumbnail_key").Find(&submissions).Error; err != nil {
		return nil, err
	}
	for _, submission := range submissions {
		objectKey := submission.ObjectKey
		add(submission.StoragePolicyID, &objectKey)
		add(submission.StoragePolicyID, submission.ThumbnailKey)
	}

	var events []appdb.Event
	if err := server.db.WithContext(ctx).Select("cover_storage_policy_id", "cover_object_key").Find(&events).Error; err != nil {
		return nil, err
	}
	for _, event := range events {
		if event.CoverStoragePolicyID != nil {
			add(*event.CoverStoragePolicyID, event.CoverObjectKey)
		}
	}
	return used, nil
}

func storageRefKey(policyID string, key string) string {
	return strings.TrimSpace(policyID) + "\x00" + strings.Trim(strings.ReplaceAll(key, "\\", "/"), "/")
}
