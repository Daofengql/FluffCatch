package http

import (
	"fmt"
	stdhttp "net/http"

	"fluffcatch/internal/auth"
	appdb "fluffcatch/internal/db"
	"fluffcatch/internal/gallery"
	"fluffcatch/internal/storage"
	"fluffcatch/internal/uploads"
)

func (server *Server) listAdminPhotos(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	defaultPageSize := server.defaultGalleryPageSize(r.Context())
	page, pageSize := parsePagination(r, defaultPageSize)
	visibility, ok := parsePhotoVisibility(r)
	if !ok {
		writeError(w, stdhttp.StatusBadRequest, "invalid visibility")
		return
	}
	result, err := server.galleryService.ListForEventPageWithOptions(r.Context(), id, gallery.ListOptions{
		Admin:        true,
		Page:         page,
		PageSize:     pageSize,
		Visibility:   visibility,
		Tag:          r.URL.Query().Get("tag"),
		Photographer: r.URL.Query().Get("photographer"),
		MediaType:    r.URL.Query().Get("mediaType"),
		Sort:         r.URL.Query().Get("sort"),
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list photos")
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"photos":     result.Items,
		"total":      result.Total,
		"page":       result.Page,
		"pageSize":   result.PageSize,
		"totalPages": result.TotalPages,
	})
}

func (server *Server) listPendingSubmissions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	submissions, err := server.uploadService.ListPending(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list submissions")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"submissions": submissions,
	})
}

func (server *Server) listEventPendingSubmissions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	submissions, err := server.uploadService.ListPendingForEvent(r.Context(), id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list submissions")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"submissions": submissions,
	})
}

func (server *Server) approveSubmissions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req uploads.BatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid batch payload")
		return
	}

	result, err := server.uploadService.ApproveBatch(r.Context(), req.SubmissionIDs, req.Visibility)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to approve submissions")
		return
	}

	writeJSON(w, stdhttp.StatusOK, result)
}

func (server *Server) deleteSubmissions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.verifyCaptchaHeader(w, r) {
		return
	}
	var req uploads.BatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid batch payload")
		return
	}

	result, err := server.uploadService.DeleteBatch(r.Context(), req.SubmissionIDs)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to delete submissions")
		return
	}

	writeJSON(w, stdhttp.StatusOK, result)
}

func (server *Server) updatePhoto(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req gallery.UpdatePhotoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid photo payload")
		return
	}
	photo, err := server.galleryService.UpdatePhoto(r.Context(), id, req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"photo": photo})
}

func (server *Server) batchUpdatePhotos(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req gallery.BatchUpdatePhotosRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid batch update payload")
		return
	}
	affected, err := server.galleryService.BatchUpdatePhotos(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"affected": affected, "message": fmt.Sprintf("updated %d photos", affected)})
}

func (server *Server) deletePhoto(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	deleted, objects, err := server.galleryService.DeletePhoto(r.Context(), id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, stdhttp.StatusNotFound, "photo not found")
		return
	}
	for _, object := range objects {
		store, err := server.storageManager.StoreForPolicy(object.PolicyID)
		if err != nil {
			continue
		}
		_ = store.Delete(r.Context(), object.Key)
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"message": "photo deleted", "deletedObjects": len(objects)})
}

func (server *Server) batchDeletePhotos(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.verifyCaptchaHeader(w, r) {
		return
	}
	var req struct {
		PhotoIDs []int64 `json:"photoIds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid batch delete payload")
		return
	}
	if len(req.PhotoIDs) == 0 {
		writeError(w, stdhttp.StatusBadRequest, "photoIds is required")
		return
	}

	deleted := 0
	var allObjects []storage.StoredObject
	for _, id := range req.PhotoIDs {
		ok, objects, err := server.galleryService.DeletePhoto(r.Context(), id)
		if err != nil || !ok {
			continue
		}
		deleted++
		allObjects = append(allObjects, objects...)
	}
	for _, object := range allObjects {
		store, err := server.storageManager.StoreForPolicy(object.PolicyID)
		if err != nil {
			continue
		}
		_ = store.Delete(r.Context(), object.Key)
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"deleted": deleted, "deletedObjects": len(allObjects)})
}

func (server *Server) adminDashboard(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	stats := map[string]any{
		"events":             int64(0),
		"photos":             int64(0),
		"pendingSubmissions": int64(0),
		"photoBytes":         int64(0),
	}
	if server.db != nil {
		var eventsCount int64
		var photosCount int64
		var pendingSubmissions int64
		var bytes int64
		_ = server.db.WithContext(r.Context()).Model(&appdb.Event{}).Count(&eventsCount).Error
		_ = server.db.WithContext(r.Context()).Model(&appdb.Photo{}).Count(&photosCount).Error
		_ = server.db.WithContext(r.Context()).Model(&appdb.Submission{}).Where("status = ?", string(uploads.SubmissionPending)).Count(&pendingSubmissions).Error
		_ = server.db.WithContext(r.Context()).Model(&appdb.Photo{}).Select("COALESCE(SUM(size_bytes), 0)").Scan(&bytes).Error
		stats["events"] = eventsCount
		stats["photos"] = photosCount
		stats["pendingSubmissions"] = pendingSubmissions
		stats["photoBytes"] = bytes
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"stats": stats})
}

func (server *Server) changePassword(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	admin, ok, err := server.currentAdmin(r)
	if err != nil || !ok {
		writeError(w, stdhttp.StatusUnauthorized, "admin authentication required")
		return
	}

	var req auth.ChangePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid password change payload")
		return
	}

	cookie, _ := r.Cookie("fluffcatch_session")
	sessionID := ""
	if cookie != nil {
		sessionID = cookie.Value
	}

	if err := server.authService.ChangePassword(r.Context(), admin.Username, req.CurrentPassword, req.NewPassword, sessionID); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]string{"message": "password changed successfully"})
}
