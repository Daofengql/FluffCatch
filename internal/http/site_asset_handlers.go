package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/url"
	"strings"

	appimage "fluffcatch/internal/image"
	"fluffcatch/internal/storage"
)

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
		if err := store.Delete(r.Context(), stored.Key); err != nil {
			slog.Warn("failed to delete logo after settings load error", "key", stored.Key, "error", err)
		}
		writeError(w, stdhttp.StatusInternalServerError, "failed to load site settings")
		return
	}
	oldLogoURL := current.Site.LogoURL
	current.Site.LogoURL = stored.URL
	updated, err := server.settingsService.UpdateSite(r.Context(), current.Site)
	if err != nil {
		if err := store.Delete(r.Context(), stored.Key); err != nil {
			slog.Warn("failed to delete logo after settings save error", "key", stored.Key, "error", err)
		}
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
		if err := store.Delete(r.Context(), stored.Key); err != nil {
			slog.Warn("failed to delete background after settings load error", "key", stored.Key, "error", err)
		}
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
		if err := store.Delete(r.Context(), stored.Key); err != nil {
			slog.Warn("failed to delete background after settings save error", "key", stored.Key, "error", err)
		}
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
	variant := strings.ToLower(strings.TrimSpace(routeParam(r, "variant")))
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
					if err := store.Delete(ctx, key); err != nil {
						slog.Warn("failed to delete site asset by policy", "policy", policyID, "key", key, "error", err)
					}
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
		if err := store.Delete(ctx, key); err != nil {
			slog.Warn("failed to delete site asset by public url", "key", key, "error", err)
		}
	}
}
