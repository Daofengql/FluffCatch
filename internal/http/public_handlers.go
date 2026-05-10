package http

import (
	"fmt"
	stdhttp "net/http"
	"strings"

	"fluffcatch/internal/events"
	"fluffcatch/internal/gallery"
	"fluffcatch/internal/uploads"
)

const hardMaxConcurrentUploads = 8

func (server *Server) publicSite(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}
	writeJSON(w, stdhttp.StatusOK, current.Site)
}

func (server *Server) publicUploadSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	upload, err := server.currentUploadSettings(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load upload settings")
		return
	}
	writeJSON(w, stdhttp.StatusOK, upload)
}

func (server *Server) health(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	payload := map[string]any{
		"status":          "ok",
		"service":         server.cfg.App.Name,
		"env":             server.cfg.App.Env,
		"storagePolicyId": server.storageManager.ActivePolicyID(),
	}

	if server.db == nil {
		payload["database"] = "not-connected"
	} else {
		sqlDB, err := server.db.DB()
		if err != nil {
			payload["database"] = "unhealthy"
		} else if err := sqlDB.PingContext(r.Context()); err != nil {
			payload["database"] = "unhealthy"
		} else {
			payload["database"] = "ok"
		}
	}

	writeJSON(w, stdhttp.StatusOK, payload)
}

func (server *Server) listPublicEvents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if server.eventListLimiter != nil && !server.eventListLimiter.Allow(clientIPFromRequest(r)) {
		writeError(w, stdhttp.StatusTooManyRequests, "too many event list requests")
		return
	}

	page, pageSize := parsePagination(r, 24)
	startDate, ok := parseOptionalDate(w, r.URL.Query().Get("startDate"), "startDate", false)
	if !ok {
		return
	}
	endDate, ok := parseOptionalDate(w, r.URL.Query().Get("endDate"), "endDate", true)
	if !ok {
		return
	}

	eventPage, err := server.eventService.ListPublicPage(r.Context(), events.ListOptions{
		CityCode:     r.URL.Query().Get("cityCode"),
		EndDate:      endDate,
		Page:         page,
		PageSize:     pageSize,
		ProvinceCode: r.URL.Query().Get("provinceCode"),
		Query:        r.URL.Query().Get("q"),
		Sort:         r.URL.Query().Get("sort"),
		StartDate:    startDate,
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list events")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"events":     eventPage.Items,
		"page":       eventPage.Page,
		"pageSize":   eventPage.PageSize,
		"total":      eventPage.Total,
		"totalPages": eventPage.TotalPages,
	})
}

func (server *Server) getPublicEvent(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	_, isAdmin, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}
	var event events.Event
	var found bool
	if isAdmin {
		event, found, err = server.eventService.GetAdmin(r.Context(), id)
	} else {
		event, found, err = server.eventService.GetPublic(r.Context(), id)
	}
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load event")
		return
	}

	if !found {
		writeError(w, stdhttp.StatusNotFound, "event not found")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"event": event,
	})
}

func (server *Server) createSubmission(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	_, isAdmin, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}

	uploadSettings, err := server.currentUploadSettings(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load upload settings")
		return
	}
	if !server.acquireUploadSlot(w, uploadSettings.MaxConcurrentUploads) {
		return
	}
	defer server.releaseUploadSlot()
	maxImageBytes := int64(uploadSettings.MaxFileSizeMB) * 1024 * 1024
	maxVideoBytes := int64(uploadSettings.MaxVideoSizeMB) * 1024 * 1024
	maxBatchBytes := max(maxImageBytes, maxVideoBytes) * int64(uploadSettings.MaxFilesPerUpload)
	r.Body = stdhttp.MaxBytesReader(w, r.Body, maxBatchBytes)
	if err := r.ParseMultipartForm(maxBatchBytes); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid multipart upload")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) > uploadSettings.MaxFilesPerUpload {
		writeError(w, stdhttp.StatusBadRequest, fmt.Sprintf("maximum of %d files per upload batch", uploadSettings.MaxFilesPerUpload))
		return
	}
	if len(files) == 0 {
		file, header, err := r.FormFile("file")
		if err == nil {
			defer file.Close()
			upload := uploads.FileUpload{
				File:             file,
				Header:           header,
				SubmissionToken:  r.FormValue("submissionToken"),
				PhotographerName: r.FormValue("photographerName"),
				Tags:             parseTagsValue(r.FormValue("tags")),
				Visibility:       r.FormValue("visibility"),
			}
			if isAdmin {
				photo, err := server.uploadService.CreateApprovedWithLimits(r.Context(), id, upload, maxImageBytes, maxVideoBytes)
				if err != nil {
					writeError(w, stdhttp.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, stdhttp.StatusCreated, map[string]any{"photos": []gallery.Photo{photo}})
				return
			}
			submission, err := server.uploadService.CreateWithLimits(r.Context(), id, upload, maxImageBytes, maxVideoBytes)
			if err != nil {
				writeError(w, stdhttp.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, stdhttp.StatusCreated, map[string]any{"submissions": []uploads.Submission{submission}})
			return
		}
		writeError(w, stdhttp.StatusBadRequest, "at least one file is required")
		return
	}

	createdSubmissions := make([]uploads.Submission, 0, len(files))
	createdPhotos := make([]gallery.Photo, 0, len(files))
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			writeError(w, stdhttp.StatusBadRequest, "failed to read upload file")
			return
		}
		upload := uploads.FileUpload{
			File:             file,
			Header:           header,
			SubmissionToken:  r.FormValue("submissionToken"),
			PhotographerName: r.FormValue("photographerName"),
			Tags:             parseTagsValue(r.FormValue("tags")),
			Visibility:       r.FormValue("visibility"),
		}
		if isAdmin {
			photo, err := server.uploadService.CreateApprovedWithLimits(r.Context(), id, upload, maxImageBytes, maxVideoBytes)
			_ = file.Close()
			if err != nil {
				writeError(w, stdhttp.StatusBadRequest, err.Error())
				return
			}
			createdPhotos = append(createdPhotos, photo)
			continue
		}
		submission, err := server.uploadService.CreateWithLimits(r.Context(), id, upload, maxImageBytes, maxVideoBytes)
		_ = file.Close()
		if err != nil {
			writeError(w, stdhttp.StatusBadRequest, err.Error())
			return
		}
		createdSubmissions = append(createdSubmissions, submission)
	}

	if isAdmin {
		writeJSON(w, stdhttp.StatusCreated, map[string]any{
			"photos": createdPhotos,
		})
		return
	}

	writeJSON(w, stdhttp.StatusCreated, map[string]any{
		"submissions": createdSubmissions,
	})
}

func (server *Server) acquireUploadSlot(w stdhttp.ResponseWriter, configuredLimit int) bool {
	if server.uploadLimiter == nil {
		return true
	}
	limit := configuredLimit
	if limit <= 0 {
		limit = 1
	}
	if limit > hardMaxConcurrentUploads {
		limit = hardMaxConcurrentUploads
	}
	if len(server.uploadLimiter) >= limit {
		writeError(w, stdhttp.StatusTooManyRequests, fmt.Sprintf("too many concurrent uploads, maximum is %d", limit))
		return false
	}
	select {
	case server.uploadLimiter <- struct{}{}:
		return true
	default:
		writeError(w, stdhttp.StatusTooManyRequests, fmt.Sprintf("too many concurrent uploads, maximum is %d", limit))
		return false
	}
}

func (server *Server) releaseUploadSlot() {
	if server.uploadLimiter == nil {
		return
	}
	select {
	case <-server.uploadLimiter:
	default:
	}
}

func (server *Server) listPublicPhotos(w stdhttp.ResponseWriter, r *stdhttp.Request) {
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
		Admin:           false,
		PrivateAccess:   server.eventPrivateAccessUnlocked(r, id),
		FingerprintHash: server.viewerFingerprintHash(w, r),
		Page:            page,
		PageSize:        pageSize,
		Visibility:      visibility,
		Tag:             r.URL.Query().Get("tag"),
		Photographer:    r.URL.Query().Get("photographer"),
		MediaType:       r.URL.Query().Get("mediaType"),
		Sort:            r.URL.Query().Get("sort"),
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list photos")
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"photos":     result.Items,
		"total":      result.Total,
		"page":       result.Page,
		"pageSize":   result.PageSize,
		"totalPages": result.TotalPages,
	})
}

func (server *Server) unlockEventPrivatePhotos(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid private access payload")
		return
	}

	verified, err := server.galleryService.VerifyEventPrivatePassword(r.Context(), id, req.Password)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to verify private password")
		return
	}
	if !verified {
		writeJSON(w, stdhttp.StatusUnauthorized, map[string]any{"unlocked": false})
		return
	}

	secure := r.TLS != nil || server.cfg.App.Env == "production"
	stdhttp.SetCookie(w, &stdhttp.Cookie{
		Name:     eventPrivateAccessCookieName(id),
		Value:    server.signEventPrivateAccessToken(id),
		Path:     "/",
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   secure,
	})
	writeJSON(w, stdhttp.StatusOK, map[string]any{"unlocked": true})
}

func (server *Server) likePhoto(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	result, err := server.galleryService.Like(r.Context(), id, server.viewerFingerprintHash(w, r))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, stdhttp.StatusNotFound, err.Error())
			return
		}
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, stdhttp.StatusOK, result)
}
