package http

import (
	"context"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	appdb "fluffcatch/internal/db"
	"fluffcatch/internal/gallery"
	appimage "fluffcatch/internal/image"
	"fluffcatch/internal/storage"

	"github.com/gin-gonic/gin"
)

func (server *Server) mountLocalMedia(r *gin.Engine) {
	r.GET("/media/photos/:id/:variant", server.ginHandler(server.servePhotoMedia))
	r.GET("/media/:policyID/*key", func(c *gin.Context) {
		policyID := c.Param("policyID")
		w := c.Writer
		r := c.Request
		localPath := server.cfg.Storage.LocalPath
		if policyID != "local" {
			current, ok := server.storageManager.ConfigForPolicy(policyID)
			if !ok {
				writeError(w, stdhttp.StatusNotFound, "media policy is unavailable")
				return
			}
			if current.Driver != "local" {
				writeError(w, stdhttp.StatusNotFound, "media route is not available for current storage driver")
				return
			}
			localPath = current.LocalPath
		}

		key := strings.TrimPrefix(c.Param("key"), "/")
		if key == "" {
			writeError(w, stdhttp.StatusBadRequest, "media key is required")
			return
		}
		cleanKey, targetPath, err := storage.LocalFilePath(localPath, key)
		if err != nil {
			writeError(w, stdhttp.StatusBadRequest, "invalid media key")
			return
		}
		allowed, err := server.canServePublicMedia(r.Context(), policyID, cleanKey)
		if err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "failed to authorize media")
			return
		}
		if !allowed {
			writeError(w, stdhttp.StatusNotFound, "media not found")
			return
		}
		stdhttp.ServeFile(w, r, targetPath)
	})
}

func (server *Server) servePhotoMedia(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	variant := routeParam(r, "variant")
	if variant != "original" && variant != "thumbnail" && variant != "blur" {
		writeError(w, stdhttp.StatusBadRequest, "invalid media variant")
		return
	}

	_, isAdmin, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}
	photo, allowed, err := server.galleryService.CanAccessPhotoWithAccess(r.Context(), id, isAdmin, false)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authorize photo")
		return
	}
	if !allowed && photo.EventID > 0 {
		photo, allowed, err = server.galleryService.CanAccessPhotoWithAccess(r.Context(), id, isAdmin, server.eventPrivateAccessUnlocked(r, photo.EventID))
		if err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "failed to authorize photo")
			return
		}
	}
	if variant == "blur" && photo.Visibility != gallery.VisibilityPrivate {
		writeError(w, stdhttp.StatusBadRequest, "blur preview is only available for locked private photos")
		return
	}
	if !allowed && !(variant == "blur" && photo.Visibility == gallery.VisibilityPrivate) {
		writeError(w, stdhttp.StatusNotFound, "media not found")
		return
	}

	key := photo.ObjectKey
	if variant == "thumbnail" {
		if photo.ThumbnailKey == "" {
			writeError(w, stdhttp.StatusNotFound, "media not found")
			return
		}
		key = photo.ThumbnailKey
	}
	if variant == "blur" {
		server.serveBlurredPhotoPreview(w, r, photo)
		return
	}
	server.serveStoredObject(w, r, photo.StoragePolicyID, key, photo.ContentType, photo.SizeBytes, photo.Visibility == gallery.VisibilityPublic)
}

func (server *Server) serveBlurredPhotoPreview(w stdhttp.ResponseWriter, r *stdhttp.Request, photo gallery.Photo) {
	key := photo.ThumbnailKey
	if key == "" {
		key = photo.ObjectKey
	}
	cacheKey := strings.Join([]string{photo.StoragePolicyID, key, photo.ContentHash}, "\x00")
	if preview, contentType, ok := server.blurCache.Get(cacheKey); ok {
		writeBlurredPreview(w, preview, contentType)
		return
	}

	store, err := server.storageManager.StoreForPolicy(photo.StoragePolicyID)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "media policy is unavailable")
		return
	}
	object, err := store.Get(r.Context(), key)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "media not found")
		return
	}
	defer object.Content.Close()

	content, err := io.ReadAll(io.LimitReader(object.Content, 8*1024*1024))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to read media")
		return
	}
	preview, contentType, err := appimage.GenerateBlurredPreviewBytes(content, 360)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to create preview")
		return
	}
	server.blurCache.Set(cacheKey, preview, contentType, 30*time.Minute)

	writeBlurredPreview(w, preview, contentType)
}

func writeBlurredPreview(w stdhttp.ResponseWriter, preview []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(preview)))
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(preview)
}

func (server *Server) serveStoredObject(w stdhttp.ResponseWriter, r *stdhttp.Request, policyID string, key string, contentType string, contentLength int64, public bool) {
	store, err := server.storageManager.StoreForPolicy(policyID)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "media policy is unavailable")
		return
	}
	object, err := store.Get(r.Context(), key)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "media not found")
		return
	}
	defer object.Content.Close()

	if contentType == "" {
		contentType = object.ContentType
	}
	if contentLength <= 0 {
		contentLength = object.ContentLength
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if contentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	if public {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	if seeker, ok := object.Content.(io.ReadSeeker); ok {
		stdhttp.ServeContent(w, r, key, time.Time{}, seeker)
		return
	}
	_, _ = io.Copy(w, object.Content)
}

func (server *Server) canServePublicMedia(ctx context.Context, policyID string, key string) (bool, error) {
	if server.db == nil {
		return true, nil
	}

	var count int64
	if err := server.db.WithContext(ctx).
		Table("photos").
		Joins("INNER JOIN events ON events.id = photos.event_id").
		Where("photos.storage_policy_id = ?", policyID).
		Where("(photos.object_key = ? OR photos.thumbnail_key = ?)", key, key).
		Where("photos.visibility = ? AND events.is_public = ?", string(gallery.VisibilityPublic), true).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	if err := server.db.WithContext(ctx).Model(&appdb.Event{}).
		Where("cover_storage_policy_id = ? AND cover_object_key = ? AND is_public = ?", policyID, key, true).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	current, err := server.settingsService.Load(ctx)
	if err != nil {
		return false, err
	}
	if strings.TrimPrefix(current.Site.LogoURL, "/") == strings.TrimPrefix(storage.MediaURL(policyID, key), "/") {
		return true, nil
	}
	if strings.HasSuffix(current.Site.LogoURL, "/"+key) {
		return true, nil
	}

	return false, nil
}
