package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"strings"

	"fluffcatch/internal/settings"
	"fluffcatch/internal/storage"
)

func (server *Server) getSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	current, err := server.settingsService.Load(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load settings")
		return
	}

	runtimeStoragePolicies := server.runtimeStoragePolicies()
	usage, err := server.settingsService.StoragePolicyUsageForIDs(r.Context(), storagePolicyIDs(runtimeStoragePolicies.Policies))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to load storage policy usage")
		return
	}

	sanitized := current.Sanitize()
	sanitized.StoragePolicies = runtimeStoragePolicies.Sanitize()
	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"settings": sanitized,
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

	policy, err := server.settingsService.ResolveStoragePolicySecrets(r.Context(), policy)
	if err != nil {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"success": false, "error": err.Error()})
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

	if err := store.Delete(r.Context(), testKey); err != nil {
		slog.Warn("failed to delete storage connection test object", "key", testKey, "error", err)
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"success": true})
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
	if upload.MaxVideoSizeMB <= 0 {
		upload.MaxVideoSizeMB = server.cfg.Upload.MaxVideoSizeMB
	}
	if upload.MaxFilesPerUpload <= 0 {
		upload.MaxFilesPerUpload = server.cfg.Upload.MaxFilesPerUpload
	}
	if upload.DefaultPageSize <= 0 {
		upload.DefaultPageSize = server.cfg.Upload.DefaultPageSize
	}
	if upload.MaxConcurrentUploads <= 0 {
		upload.MaxConcurrentUploads = server.cfg.Upload.MaxConcurrentUploads
	}
	return upload, nil
}

func (server *Server) defaultGalleryPageSize(ctx context.Context) int {
	upload, err := server.currentUploadSettings(ctx)
	if err != nil || upload.DefaultPageSize <= 0 {
		if server.cfg.Upload.DefaultPageSize > 0 {
			return server.cfg.Upload.DefaultPageSize
		}
		return 24
	}
	return upload.DefaultPageSize
}
