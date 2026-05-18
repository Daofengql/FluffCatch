package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	stdhttp "net/http"
	"strings"

	appdb "fluffcatch/internal/db"
	"fluffcatch/internal/events"
	"fluffcatch/internal/storage"
)

func (server *Server) listAdminEvents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	eventList, err := server.eventService.ListAdmin(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list events")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"events": eventList,
	})
}

func (server *Server) createAdminEvent(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req events.CreateEventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid event payload")
		return
	}

	event, err := server.eventService.Create(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusCreated, map[string]any{"event": event})
}

func (server *Server) updateAdminEvent(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	var req events.CreateEventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid event payload")
		return
	}

	event, err := server.eventService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{"event": event})
}

func (server *Server) deleteAdminEvent(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.verifyCaptchaHeader(w, r) {
		return
	}
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	deleted, objects, err := server.eventService.Delete(r.Context(), id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, stdhttp.StatusNotFound, "event not found")
		return
	}

	for _, object := range objects {
		store, err := server.storageManager.StoreForPolicy(object.PolicyID)
		if err != nil {
			continue
		}
		_ = store.Delete(r.Context(), object.Key)
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{"message": "event deleted", "deletedObjects": len(objects)})
}

func (server *Server) uploadEventCover(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	maxBytes := int64(server.cfg.Upload.MaxSizeMB) * 1024 * 1024
	r.Body = stdhttp.MaxBytesReader(w, r.Body, maxBytes)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "cover file is required")
		return
	}
	defer file.Close()

	if header.Size > maxBytes {
		writeError(w, stdhttp.StatusBadRequest, "cover file exceeds maximum upload size")
		return
	}

	limited := io.LimitReader(file, maxBytes+1)
	head := make([]byte, 512)
	n, err := io.ReadFull(limited, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		writeError(w, stdhttp.StatusBadRequest, "failed to read cover file")
		return
	}
	head = head[:n]
	detectedType := stdhttp.DetectContentType(head)
	if !strings.HasPrefix(detectedType, "image/") {
		writeError(w, stdhttp.StatusBadRequest, "cover must be an image")
		return
	}

	content, err := io.ReadAll(io.MultiReader(bytes.NewReader(head), limited))
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "failed to read cover file")
		return
	}
	if len(content) == 0 {
		writeError(w, stdhttp.StatusBadRequest, "cover file is empty")
		return
	}
	if int64(len(content)) > maxBytes {
		writeError(w, stdhttp.StatusBadRequest, "cover file exceeds maximum upload size")
		return
	}

	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])
	objectKey := coverObjectKey(id, contentHash, header.Filename)

	store, err := server.storageManager.ActiveStore()
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	stored, err := store.Put(r.Context(), storage.Object{
		Key:         objectKey,
		Content:     bytes.NewReader(content),
		ContentType: detectedType,
		Size:        int64(len(content)),
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	result := server.db.WithContext(r.Context()).Model(&appdb.Event{}).Where("id = ?", id).Updates(map[string]any{
		"cover_storage_policy_id": stored.PolicyID,
		"cover_object_key":        stored.Key,
	})
	if result.Error != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to update event cover")
		return
	}
	if result.RowsAffected == 0 {
		_ = store.Delete(r.Context(), stored.Key)
		writeError(w, stdhttp.StatusNotFound, "event not found")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{"policyId": stored.PolicyID, "objectKey": stored.Key, "url": stored.URL})
}

func (server *Server) setEventCoverFromPhoto(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	eventID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		PhotoID int64 `json:"photoId"`
	}
	if err := decodeJSON(r, &req); err != nil || req.PhotoID <= 0 {
		writeError(w, stdhttp.StatusBadRequest, "invalid cover photo payload")
		return
	}
	var photo appdb.Photo
	err := server.db.WithContext(r.Context()).
		Where("id = ? AND event_id = ?", req.PhotoID, eventID).
		Take(&photo).Error
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "photo not found")
		return
	}
	if photo.Visibility != "public" {
		writeError(w, stdhttp.StatusBadRequest, "only public photos can be used as event cover")
		return
	}
	coverKey := photo.ObjectKey
	if photo.ThumbnailKey != nil && strings.TrimSpace(*photo.ThumbnailKey) != "" {
		coverKey = *photo.ThumbnailKey
	}
	result := server.db.WithContext(r.Context()).Model(&appdb.Event{}).Where("id = ?", eventID).Updates(map[string]any{
		"cover_storage_policy_id": photo.StoragePolicyID,
		"cover_object_key":        coverKey,
	})
	if result.Error != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to update event cover")
		return
	}
	store, err := server.storageManager.StoreForPolicy(photo.StoragePolicyID)
	url := storage.MediaURL(photo.StoragePolicyID, coverKey)
	if err == nil {
		url = store.PublicURL(coverKey)
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"policyId": photo.StoragePolicyID, "objectKey": coverKey, "url": url})
}
