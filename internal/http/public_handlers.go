package http

import (
	"fmt"
	stdhttp "net/http"
	"strings"

	"fluffcatch/internal/events"
	"fluffcatch/internal/gallery"
	"fluffcatch/internal/uploads"
)

func (server *Server) publicSite(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}
	writeJSON(w, stdhttp.StatusOK, current.Site)
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
	eventList, err := server.eventService.ListPublic(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list events")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"events": eventList,
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
				File:               file,
				Header:             header,
				SubmissionPassword: r.FormValue("submissionPassword"),
				PhotographerName:   r.FormValue("photographerName"),
				Tags:               parseTagsValue(r.FormValue("tags")),
				Visibility:         r.FormValue("visibility"),
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
			File:               file,
			Header:             header,
			SubmissionPassword: r.FormValue("submissionPassword"),
			PhotographerName:   r.FormValue("photographerName"),
			Tags:               parseTagsValue(r.FormValue("tags")),
			Visibility:         r.FormValue("visibility"),
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

func (server *Server) listPublicPhotos(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	page, pageSize := parsePagination(r)
	result, err := server.galleryService.ListForEventPageWithAccess(r.Context(), id, false, server.eventPrivateAccessUnlocked(r, id), viewerFingerprintHash(r), page, pageSize)
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

	result, err := server.galleryService.Like(r.Context(), id, viewerFingerprintHash(r))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, stdhttp.StatusNotFound, err.Error())
			return
		}
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, result)
}
