package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	stdhttp "net/http"
	"net/url"
	"os"
	urlpath "path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/config"
	"fluffcatch/internal/events"
	"fluffcatch/internal/gallery"
	appimage "fluffcatch/internal/image"
	"fluffcatch/internal/settings"
	"fluffcatch/internal/storage"
	"fluffcatch/internal/uploads"
	frontend "fluffcatch/www"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	cfg             config.Config
	db              *sql.DB
	storageManager  *storage.Manager
	settingsService *settings.Service
	authService     *auth.Service
	captchaStore    *auth.CaptchaStore
	eventService    *events.Service
	uploadService   *uploads.Service
	galleryService  *gallery.Service
	loginLimiter    *rateLimiter
	captchaLimiter  *rateLimiter
	blurCache       *blurPreviewCache
}

type blurPreviewCache struct {
	mu      sync.Mutex
	entries map[string]blurPreviewEntry
}

type blurPreviewEntry struct {
	content     []byte
	contentType string
	expiresAt   time.Time
}

func newBlurPreviewCache() *blurPreviewCache {
	return &blurPreviewCache{entries: map[string]blurPreviewEntry{}}
}

func (cache *blurPreviewCache) Get(key string) ([]byte, string, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, ok := cache.entries[key]
	if !ok {
		return nil, "", false
	}
	if time.Now().After(entry.expiresAt) {
		delete(cache.entries, key)
		return nil, "", false
	}
	return entry.content, entry.contentType, true
}

func (cache *blurPreviewCache) Set(key string, content []byte, contentType string, ttl time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.entries[key] = blurPreviewEntry{
		content:     append([]byte(nil), content...),
		contentType: contentType,
		expiresAt:   time.Now().Add(ttl),
	}
	if len(cache.entries) <= 256 {
		return
	}

	now := time.Now()
	for key, entry := range cache.entries {
		if now.After(entry.expiresAt) {
			delete(cache.entries, key)
		}
	}
	if len(cache.entries) <= 256 {
		return
	}
	for key := range cache.entries {
		delete(cache.entries, key)
		if len(cache.entries) <= 256 {
			return
		}
	}
}

func NewServer(cfg config.Config, dbConn *sql.DB, storageManager *storage.Manager, settingsService *settings.Service) *Server {
	return &Server{
		cfg:             cfg,
		db:              dbConn,
		storageManager:  storageManager,
		settingsService: settingsService,
		authService:     auth.NewService(dbConn, cfg.Auth.AdminUsername),
		captchaStore:    auth.NewCaptchaStore(),
		eventService:    events.NewService(dbConn, storageManager),
		uploadService:   uploads.NewService(dbConn, storageManager, cfg.Upload.MaxSizeMB),
		galleryService:  gallery.NewService(dbConn, storageManager),
		loginLimiter:    newRateLimiter(1, 5),
		captchaLimiter:  newRateLimiter(2, 10),
		blurCache:       newBlurPreviewCache(),
	}
}

func (server *Server) Routes() stdhttp.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/health", server.health)

		api.Route("/auth", func(authRouter chi.Router) {
			authRouter.Get("/captcha", server.captcha)
			authRouter.Post("/login", server.login)
			authRouter.Post("/logout", server.logout)
			authRouter.Get("/me", server.me)
		})

		api.Get("/events", server.listPublicEvents)
		api.Get("/site", server.publicSite)
		api.Get("/events/{id}", server.getPublicEvent)
		api.Get("/events/{id}/photos", server.listPublicPhotos)
		api.Post("/events/{id}/private-access", server.unlockEventPrivatePhotos)
		api.Post("/events/{id}/submissions", server.createSubmission)
		api.Post("/photos/{id}/like", server.likePhoto)

		api.Route("/admin", func(admin chi.Router) {
			admin.Use(server.requireAdmin)
			admin.Get("/dashboard", server.adminDashboard)
			admin.Get("/events", server.listAdminEvents)
			admin.Post("/events", server.createAdminEvent)
			admin.Put("/events/{id}", server.updateAdminEvent)
			admin.Delete("/events/{id}", server.deleteAdminEvent)
			admin.Post("/events/{id}/cover", server.uploadEventCover)
			admin.Get("/events/{id}/photos", server.listAdminPhotos)
			admin.Get("/events/{id}/submissions", server.listEventPendingSubmissions)
			admin.Get("/submissions", server.listPendingSubmissions)
			admin.Post("/submissions/batch-approve", server.approveSubmissions)
			admin.Post("/submissions/batch-delete", server.deleteSubmissions)
			admin.Put("/photos/{id}", server.updatePhoto)
			admin.Delete("/photos/{id}", server.deletePhoto)
			admin.Post("/photos/batch-delete", server.batchDeletePhotos)
			admin.Get("/settings", server.getSettings)
			admin.Put("/settings/storage", server.updateStorageSettings)
			admin.Post("/settings/storage/test", server.testStorageConnection)
			admin.Put("/settings/oidc", server.updateOIDCSettings)
			admin.Put("/settings/site", server.updateSiteSettings)
			admin.Post("/settings/site/logo", server.uploadSiteLogo)
			admin.Delete("/settings/site/logo", server.clearSiteLogo)
			admin.Post("/settings/site/background/{variant}", server.uploadSiteBackground)
			admin.Delete("/settings/site/background/{variant}", server.clearSiteBackground)
			admin.Put("/settings/upload", server.updateUploadSettings)
		})
	})

	server.mountLocalMedia(r)
	server.mountStaticApp(r)

	return r
}

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
	} else if err := server.db.PingContext(r.Context()); err != nil {
		payload["database"] = "unhealthy"
	} else {
		payload["database"] = "ok"
	}

	writeJSON(w, stdhttp.StatusOK, payload)
}

func (server *Server) captcha(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.captchaLimiter.Allow(clientIP(r.RemoteAddr)) {
		writeError(w, stdhttp.StatusTooManyRequests, "too many captcha requests, please slow down")
		return
	}
	challenge, err := server.captchaStore.NewChallenge(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to create captcha")
		return
	}

	writeJSON(w, stdhttp.StatusOK, challenge)
}

func (server *Server) login(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.loginLimiter.Allow(clientIP(r.RemoteAddr)) {
		writeError(w, stdhttp.StatusTooManyRequests, "too many login attempts, please slow down")
		return
	}
	var req auth.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid login payload")
		return
	}

	if !server.captchaStore.Verify(r.Context(), req.CaptchaID, req.CaptchaAnswer) {
		writeError(w, stdhttp.StatusBadRequest, "invalid captcha")
		return
	}

	result := server.authService.Login(r.Context(), req)
	if !result.Authenticated {
		writeJSON(w, stdhttp.StatusUnauthorized, result)
		return
	}

	sessionID, expiresAt, err := server.authService.CreateSession(r.Context(), req.Username, 30*24*time.Hour)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to create session")
		return
	}

	secure := r.TLS != nil || server.cfg.App.Env == "production"
	stdhttp.SetCookie(w, &stdhttp.Cookie{
		Name:     "fluffcatch_session",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   secure,
	})

	writeJSON(w, stdhttp.StatusOK, result)
}

func (server *Server) logout(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if cookie, err := r.Cookie("fluffcatch_session"); err == nil {
		_ = server.authService.Logout(r.Context(), cookie.Value)
	}
	secure := r.TLS != nil || server.cfg.App.Env == "production"
	stdhttp.SetCookie(w, &stdhttp.Cookie{
		Name:     "fluffcatch_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   secure,
	})
	writeJSON(w, stdhttp.StatusOK, map[string]string{
		"message": "logged out",
	})
}

func (server *Server) me(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	user, ok, err := server.currentAdmin(r)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
		return
	}
	if !ok {
		writeJSON(w, stdhttp.StatusOK, auth.MeResponse{Authenticated: false})
		return
	}

	writeJSON(w, stdhttp.StatusOK, auth.MeResponse{Authenticated: true, Username: user.Username})
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
	maxFileBytes := int64(uploadSettings.MaxFileSizeMB) * 1024 * 1024
	maxBatchBytes := maxFileBytes * int64(uploadSettings.MaxFilesPerUpload)
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
			}
			if isAdmin {
				photo, err := server.uploadService.CreateApprovedWithLimit(r.Context(), id, upload, maxFileBytes)
				if err != nil {
					writeError(w, stdhttp.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, stdhttp.StatusCreated, map[string]any{"photos": []gallery.Photo{photo}})
				return
			}
			submission, err := server.uploadService.CreateWithLimit(r.Context(), id, upload, maxFileBytes)
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
		}
		if isAdmin {
			photo, err := server.uploadService.CreateApprovedWithLimit(r.Context(), id, upload, maxFileBytes)
			_ = file.Close()
			if err != nil {
				writeError(w, stdhttp.StatusBadRequest, err.Error())
				return
			}
			createdPhotos = append(createdPhotos, photo)
			continue
		}
		submission, err := server.uploadService.CreateWithLimit(r.Context(), id, upload, maxFileBytes)
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
	result, err := server.db.ExecContext(r.Context(), "UPDATE events SET cover_storage_policy_id = ?, cover_object_key = ? WHERE id = ?", stored.PolicyID, stored.Key, id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to update event cover")
		return
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		_ = store.Delete(r.Context(), stored.Key)
		writeError(w, stdhttp.StatusNotFound, "event not found")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{"policyId": stored.PolicyID, "objectKey": stored.Key, "url": stored.URL})
}

func (server *Server) listAdminPhotos(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	page, pageSize := parsePagination(r)
	result, err := server.galleryService.ListForEventPage(r.Context(), id, true, "", page, pageSize)
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

	result, err := server.uploadService.ApproveBatch(r.Context(), req.SubmissionIDs)
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

func (server *Server) deletePhoto(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if !server.verifyCaptchaHeader(w, r) {
		return
	}
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
		_ = server.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM events").Scan(&eventsCount)
		_ = server.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM photos").Scan(&photosCount)
		_ = server.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM submissions WHERE status = 'pending'").Scan(&pendingSubmissions)
		_ = server.db.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(size_bytes), 0) FROM photos").Scan(&bytes)
		stats["events"] = eventsCount
		stats["photos"] = photosCount
		stats["pendingSubmissions"] = pendingSubmissions
		stats["photoBytes"] = bytes
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"stats": stats})
}

func (server *Server) getSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load settings")
		return
	}

	usage, err := server.settingsService.StoragePolicyUsage(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load storage policy usage")
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"settings": current.Sanitize(),
		"usage":    usage,
	})
}

func (server *Server) updateStorageSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req settings.StoragePoliciesSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid storage settings payload")
		return
	}

	updated, err := server.settingsService.UpdateStoragePolicies(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	if err := server.storageManager.Reconfigure(updated.ActivePolicyID, storageConfigsFromPolicies(updated.Policies)); err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"storagePolicies": updated.Sanitize(),
		"usage":           server.storagePolicyUsageOrEmpty(r),
		"message":         "storage policies updated",
	})
}

func (server *Server) testStorageConnection(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var policy settings.StoragePolicy
	if err := decodeJSON(r, &policy); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid storage policy payload")
		return
	}

	normalized, err := settings.NormalizeStoragePolicy(policy)
	if err != nil {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}

	config := storageConfigFromPolicy(normalized)
	store, err := storage.New(config)
	if err != nil {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}

	testKey := "_test/connection_test.bin"
	_, err = store.Put(r.Context(), storage.Object{
		Key:         testKey,
		Content:     strings.NewReader("ok"),
		ContentType: "application/octet-stream",
		Size:        2,
	})
	if err != nil {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"success": false, "error": "upload test failed: " + err.Error()})
		return
	}

	_ = store.Delete(r.Context(), testKey)
	writeJSON(w, stdhttp.StatusOK, map[string]any{"success": true})
}

func (server *Server) updateOIDCSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req settings.OIDCSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid oidc settings payload")
		return
	}

	updated, err := server.settingsService.UpdateOIDC(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"oidc":    updated.Sanitize(),
		"message": "oidc settings updated",
	})
}

func (server *Server) updateSiteSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req settings.SiteSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid site settings payload")
		return
	}
	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}
	req.LogoURL = current.Site.LogoURL
	req.PublicBackgroundDesktopURL = current.Site.PublicBackgroundDesktopURL
	req.PublicBackgroundMobileURL = current.Site.PublicBackgroundMobileURL

	updated, err := server.settingsService.UpdateSite(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"site": updated, "message": "site settings updated"})
}

func (server *Server) updateUploadSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req settings.UploadSettings
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid upload settings payload")
		return
	}

	updated, err := server.settingsService.UpdateUpload(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]any{"upload": updated, "message": "upload settings updated"})
}

func (server *Server) currentUploadSettings(ctx context.Context) (settings.UploadSettings, error) {
	current, err := server.settingsService.Load(ctx)
	if err != nil {
		return settings.UploadSettings{}, err
	}
	upload := current.Upload
	if upload.MaxFileSizeMB <= 0 {
		upload.MaxFileSizeMB = server.cfg.Upload.MaxSizeMB
	}
	if upload.MaxFilesPerUpload <= 0 {
		upload.MaxFilesPerUpload = server.cfg.Upload.MaxFilesPerUpload
	}
	return upload, nil
}

func (server *Server) clearSiteLogo(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}
	oldLogoURL := current.Site.LogoURL
	current.Site.LogoURL = ""

	updated, err := server.settingsService.UpdateSite(r.Context(), current.Site)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to clear site logo")
		return
	}
	server.deleteSiteAsset(r.Context(), oldLogoURL)

	writeJSON(w, stdhttp.StatusOK, map[string]any{"site": updated, "message": "site logo cleared"})
}

func (server *Server) uploadSiteLogo(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	maxBytes := int64(server.cfg.Upload.MaxSizeMB) * 1024 * 1024
	r.Body = stdhttp.MaxBytesReader(w, r.Body, maxBytes)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "logo file is required")
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		writeError(w, stdhttp.StatusBadRequest, "logo file is empty")
		return
	}
	if header.Size > maxBytes {
		writeError(w, stdhttp.StatusBadRequest, "logo file exceeds maximum upload size")
		return
	}

	limited := io.LimitReader(file, maxBytes+1)
	head := make([]byte, 512)
	n, err := io.ReadFull(limited, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		writeError(w, stdhttp.StatusBadRequest, "failed to read logo file")
		return
	}
	head = head[:n]
	detectedType := stdhttp.DetectContentType(head)
	if !strings.HasPrefix(detectedType, "image/") {
		writeError(w, stdhttp.StatusBadRequest, "logo must be an image")
		return
	}

	content, err := io.ReadAll(io.MultiReader(bytes.NewReader(head), limited))
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "failed to read logo file")
		return
	}
	if len(content) == 0 {
		writeError(w, stdhttp.StatusBadRequest, "logo file is empty")
		return
	}
	if int64(len(content)) > maxBytes {
		writeError(w, stdhttp.StatusBadRequest, "logo file exceeds maximum upload size")
		return
	}

	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])
	objectKey := logoObjectKey(contentHash, header.Filename)

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

	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		_ = store.Delete(r.Context(), stored.Key)
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}
	oldLogoURL := current.Site.LogoURL
	current.Site.LogoURL = stored.URL
	updated, err := server.settingsService.UpdateSite(r.Context(), current.Site)
	if err != nil {
		_ = store.Delete(r.Context(), stored.Key)
		writeError(w, stdhttp.StatusInternalServerError, "failed to save site logo")
		return
	}
	server.deleteSiteAsset(r.Context(), oldLogoURL)

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"site":    updated,
		"url":     stored.URL,
		"message": "site logo uploaded",
	})
}

func (server *Server) clearSiteBackground(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	variant, ok := parseSiteBackgroundVariant(w, r)
	if !ok {
		return
	}

	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}

	var oldURL string
	switch variant {
	case "desktop":
		oldURL = current.Site.PublicBackgroundDesktopURL
		current.Site.PublicBackgroundDesktopURL = ""
	case "mobile":
		oldURL = current.Site.PublicBackgroundMobileURL
		current.Site.PublicBackgroundMobileURL = ""
	}

	updated, err := server.settingsService.UpdateSite(r.Context(), current.Site)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to clear site background")
		return
	}
	server.deleteSiteAsset(r.Context(), oldURL)

	writeJSON(w, stdhttp.StatusOK, map[string]any{"site": updated, "message": "site background cleared"})
}

func (server *Server) uploadSiteBackground(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	variant, ok := parseSiteBackgroundVariant(w, r)
	if !ok {
		return
	}

	maxBytes := int64(15 * 1024 * 1024)
	if configured := int64(server.cfg.Upload.MaxSizeMB) * 1024 * 1024; configured > 0 && configured < maxBytes {
		maxBytes = configured
	}
	r.Body = stdhttp.MaxBytesReader(w, r.Body, maxBytes)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "background file is required")
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		writeError(w, stdhttp.StatusBadRequest, "background file is empty")
		return
	}
	if header.Size > maxBytes {
		writeError(w, stdhttp.StatusBadRequest, "background file exceeds maximum upload size")
		return
	}

	limited := io.LimitReader(file, maxBytes+1)
	head := make([]byte, 512)
	n, err := io.ReadFull(limited, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		writeError(w, stdhttp.StatusBadRequest, "failed to read background file")
		return
	}
	head = head[:n]
	detectedType := stdhttp.DetectContentType(head)
	if !strings.HasPrefix(detectedType, "image/") {
		writeError(w, stdhttp.StatusBadRequest, "background must be an image")
		return
	}

	content, err := io.ReadAll(io.MultiReader(bytes.NewReader(head), limited))
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "failed to read background file")
		return
	}
	if len(content) == 0 {
		writeError(w, stdhttp.StatusBadRequest, "background file is empty")
		return
	}
	if int64(len(content)) > maxBytes {
		writeError(w, stdhttp.StatusBadRequest, "background file exceeds maximum upload size")
		return
	}

	targetWidth, targetHeight := 1920, 1080
	if variant == "mobile" {
		targetWidth, targetHeight = 1080, 1920
	}
	processed, contentType, err := appimage.GenerateCoverJPEGBytes(content, targetWidth, targetHeight, 82)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "failed to process background image")
		return
	}

	hash := sha256.Sum256(processed)
	contentHash := hex.EncodeToString(hash[:])
	objectKey := siteBackgroundObjectKey(variant, contentHash)

	store, err := server.storageManager.ActiveStore()
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	stored, err := store.Put(r.Context(), storage.Object{
		Key:         objectKey,
		Content:     bytes.NewReader(processed),
		ContentType: contentType,
		Size:        int64(len(processed)),
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}

	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		_ = store.Delete(r.Context(), stored.Key)
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}

	var oldURL string
	switch variant {
	case "desktop":
		oldURL = current.Site.PublicBackgroundDesktopURL
		current.Site.PublicBackgroundDesktopURL = stored.URL
	case "mobile":
		oldURL = current.Site.PublicBackgroundMobileURL
		current.Site.PublicBackgroundMobileURL = stored.URL
	}
	updated, err := server.settingsService.UpdateSite(r.Context(), current.Site)
	if err != nil {
		_ = store.Delete(r.Context(), stored.Key)
		writeError(w, stdhttp.StatusInternalServerError, "failed to save site background")
		return
	}
	server.deleteSiteAsset(r.Context(), oldURL)

	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"site":    updated,
		"url":     stored.URL,
		"message": "site background uploaded",
		"width":   targetWidth,
		"height":  targetHeight,
	})
}

func parseSiteBackgroundVariant(w stdhttp.ResponseWriter, r *stdhttp.Request) (string, bool) {
	variant := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "variant")))
	if variant != "desktop" && variant != "mobile" {
		writeError(w, stdhttp.StatusBadRequest, "background variant must be desktop or mobile")
		return "", false
	}
	return variant, true
}

func (server *Server) deleteSiteAsset(ctx context.Context, assetURL string) {
	if assetURL == "" {
		return
	}
	if strings.HasPrefix(assetURL, "/media/") {
		parts := strings.SplitN(strings.TrimPrefix(assetURL, "/media/"), "/", 2)
		if len(parts) == 2 {
			policyID, policyErr := url.PathUnescape(parts[0])
			key, keyErr := url.PathUnescape(parts[1])
			if policyErr == nil && keyErr == nil {
				if store, err := server.storageManager.StoreForPolicy(policyID); err == nil {
					_ = store.Delete(ctx, key)
					return
				}
			}
		}
	}

	store, err := server.storageManager.ActiveStore()
	if err != nil {
		return
	}
	publicURL := strings.TrimRight(store.PublicURL(""), "/")
	if strings.HasPrefix(assetURL, publicURL+"/") {
		key := strings.TrimPrefix(assetURL, publicURL+"/")
		_ = store.Delete(ctx, key)
	}
}

func (server *Server) storagePolicyUsageOrEmpty(r *stdhttp.Request) map[string]settings.PolicyUsage {
	usage, err := server.settingsService.StoragePolicyUsage(r.Context())
	if err != nil {
		return map[string]settings.PolicyUsage{}
	}

	return usage
}

func (server *Server) requireAdmin(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		_, ok, err := server.currentAdmin(r)
		if err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "failed to authenticate session")
			return
		}
		if !ok {
			writeError(w, stdhttp.StatusUnauthorized, "admin authentication required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (server *Server) currentAdmin(r *stdhttp.Request) (auth.AdminUser, bool, error) {
	cookie, err := r.Cookie("fluffcatch_session")
	if err != nil {
		return auth.AdminUser{}, false, nil
	}
	return server.authService.AuthenticateSession(r.Context(), cookie.Value)
}

const eventPrivateAccessTokenTTL = 24 * time.Hour

func (server *Server) eventPrivateAccessUnlocked(r *stdhttp.Request, eventID int64) bool {
	cookie, err := r.Cookie(eventPrivateAccessCookieName(eventID))
	if err != nil {
		return false
	}
	return server.verifyEventPrivateAccessToken(cookie.Value, eventID)
}

func eventPrivateAccessCookieName(eventID int64) string {
	return fmt.Sprintf("fluffcatch_private_%d", eventID)
}

func (server *Server) signEventPrivateAccessToken(eventID int64) string {
	issuedAt := strconv.FormatInt(time.Now().Unix(), 10)
	payload := strings.Join([]string{"v1", strconv.FormatInt(eventID, 10), issuedAt}, ".")
	signature := server.signEventPrivateAccessPayload(payload)
	return payload + "." + signature
}

func (server *Server) verifyEventPrivateAccessToken(token string, eventID int64) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != strconv.FormatInt(eventID, 10) {
		return false
	}
	issuedUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return false
	}
	issuedAt := time.Unix(issuedUnix, 0)
	if time.Since(issuedAt) > eventPrivateAccessTokenTTL || issuedAt.After(time.Now().Add(5*time.Minute)) {
		return false
	}

	payload := strings.Join(parts[:3], ".")
	expected := server.signEventPrivateAccessPayload(payload)
	return hmac.Equal([]byte(expected), []byte(parts[3]))
}

func (server *Server) signEventPrivateAccessPayload(payload string) string {
	secret := server.cfg.Auth.SessionSecret
	if secret == "" {
		secret = "fluffcatch-private-access-development"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (server *Server) mountLocalMedia(r chi.Router) {
	r.Get("/media/photos/{id}/{variant}", server.servePhotoMedia)
	r.Get("/media/{policyID}/*", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		policyID := chi.URLParam(r, "policyID")
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

		key := chi.URLParam(r, "*")
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
	variant := chi.URLParam(r, "variant")
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
	_, _ = io.Copy(w, object.Content)
}

func (server *Server) canServePublicMedia(ctx context.Context, policyID string, key string) (bool, error) {
	if server.db == nil {
		return true, nil
	}

	var exists int
	err := server.db.QueryRowContext(ctx, `
		SELECT 1
		FROM photos
		INNER JOIN events ON events.id = photos.event_id
		WHERE photos.storage_policy_id = ?
			AND (photos.object_key = ? OR photos.thumbnail_key = ?)
			AND photos.visibility = 'public'
			AND events.is_public = true
		LIMIT 1
	`, policyID, key, key).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}

	err = server.db.QueryRowContext(ctx, `
		SELECT 1
		FROM events
		WHERE cover_storage_policy_id = ?
			AND cover_object_key = ?
			AND is_public = true
		LIMIT 1
	`, policyID, key).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err != sql.ErrNoRows {
		return false, err
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

func (server *Server) mountStaticApp(r chi.Router) {
	switch server.cfg.Frontend.Mode {
	case "external", "disabled":
		return
	case "embedded":
		if dist, ok := frontend.Dist(); ok {
			mountStaticAppFS(r, dist)
		}
		return
	case "disk":
		server.mountDiskStaticApp(r)
		return
	case "auto":
		if dist, ok := frontend.Dist(); ok && mountStaticAppFS(r, dist) {
			return
		}
		server.mountDiskStaticApp(r)
	}
}

func storageConfigsFromPolicies(policies []settings.StoragePolicy) []storage.Config {
	configs := make([]storage.Config, 0, len(policies))
	for _, policy := range policies {
		configs = append(configs, storageConfigFromPolicy(policy))
	}

	return configs
}

func storageConfigFromPolicy(policy settings.StoragePolicy) storage.Config {
	return storage.Config{
		PolicyID:      policy.ID,
		Name:          policy.Name,
		Driver:        policy.Driver,
		LocalPath:     policy.LocalPath,
		PublicPrefix:  policy.PublicPrefix,
		PublicBaseURL: policy.PublicBaseURL,
		S3: storage.S3Config{
			Endpoint:  policy.S3.Endpoint,
			Bucket:    policy.S3.Bucket,
			Region:    policy.S3.Region,
			AccessKey: policy.S3.AccessKey,
			SecretKey: policy.S3.SecretKey,
			UseSSL:    policy.S3.UseSSL,
			AccountID: policy.S3.AccountID,
		},
	}
}

func (server *Server) mountDiskStaticApp(r chi.Router) {
	staticRoot := server.cfg.Frontend.StaticRoot
	indexPath := filepath.Join(staticRoot, "index.html")
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return
	}

	fileServer := stdhttp.FileServer(stdhttp.Dir(staticRoot))
	r.Get("/*", func(w stdhttp.ResponseWriter, request *stdhttp.Request) {
		cleanPath := urlpath.Clean(request.URL.Path)
		if strings.HasPrefix(cleanPath, "/api/") {
			writeError(w, stdhttp.StatusNotFound, "api route not found")
			return
		}

		targetPath := filepath.Join(staticRoot, filepath.FromSlash(strings.TrimPrefix(cleanPath, "/")))
		if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, request)
			return
		}

		stdhttp.ServeFile(w, request, indexPath)
	})
}

func mountStaticAppFS(r chi.Router, dist fs.FS) bool {
	index, err := dist.Open("index.html")
	if err != nil {
		return false
	}
	_ = index.Close()

	fileServer := stdhttp.FileServer(stdhttp.FS(dist))
	r.Get("/*", func(w stdhttp.ResponseWriter, request *stdhttp.Request) {
		cleanPath := urlpath.Clean(request.URL.Path)
		if strings.HasPrefix(cleanPath, "/api/") {
			writeError(w, stdhttp.StatusNotFound, "api route not found")
			return
		}

		target := strings.TrimPrefix(cleanPath, "/")
		if target != "" {
			if info, err := fs.Stat(dist, target); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, request)
				return
			}
		}

		stdhttp.ServeFileFS(w, request, dist, "index.html")
	})

	return true
}

func parseIDParam(w stdhttp.ResponseWriter, r *stdhttp.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, stdhttp.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return 0, false
	}

	return id, true
}

func parsePagination(r *stdhttp.Request) (int, int) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil || pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func parseTagsValue(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	return fields
}

func sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	if strings.TrimSpace(filename) == "" {
		return "upload.bin"
	}
	return filename
}

func coverObjectKey(eventID int64, contentHash string, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("events/%d/cover/%s%s", eventID, contentHash, ext)
}

func logoObjectKey(contentHash string, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("site/logo/%s%s", contentHash, ext)
}

func siteBackgroundObjectKey(variant string, contentHash string) string {
	return fmt.Sprintf("site/backgrounds/%s/%s.jpg", variant, contentHash)
}

func viewerFingerprintHash(r *stdhttp.Request) string {
	ip := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			ip = first
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		ip = realIP
	}

	sum := sha256.Sum256([]byte(strings.Join([]string{
		ip,
		r.UserAgent(),
		strings.TrimSpace(r.Header.Get("Accept-Language")),
	}, "|")))
	return hex.EncodeToString(sum[:])
}
