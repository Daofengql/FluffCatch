package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	stdhttp "net/http"
	"os"
	urlpath "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fluffcatch/internal/gallery"
	frontend "fluffcatch/www"

	"github.com/gin-gonic/gin"
)

const viewerCookieName = "fluffcatch_viewer"
const viewerCookieMaxAge = 365 * 24 * 60 * 60

func (server *Server) mountStaticApp(r *gin.Engine) {
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

func (server *Server) mountDiskStaticApp(r *gin.Engine) {
	staticRoot := server.cfg.Frontend.StaticRoot
	indexPath := filepath.Join(staticRoot, "index.html")
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return
	}

	fileServer := stdhttp.FileServer(stdhttp.Dir(staticRoot))
	r.NoRoute(func(c *gin.Context) {
		w := c.Writer
		request := c.Request
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

func mountStaticAppFS(r *gin.Engine, dist fs.FS) bool {
	index, err := dist.Open("index.html")
	if err != nil {
		return false
	}
	_ = index.Close()

	fileServer := stdhttp.FileServer(stdhttp.FS(dist))
	r.NoRoute(func(c *gin.Context) {
		w := c.Writer
		request := c.Request
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
	raw := routeParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, stdhttp.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return 0, false
	}

	return id, true
}

func parsePagination(r *stdhttp.Request, defaultPageSize int) (int, int) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil || pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func parseOptionalDate(w stdhttp.ResponseWriter, value string, name string, endOfDay bool) (*time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return nil, false
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return &parsed, true
}

func parsePhotoVisibility(r *stdhttp.Request) (gallery.Visibility, bool) {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("visibility"))) {
	case "", "all":
		return "", true
	case "public":
		return gallery.VisibilityPublic, true
	case "private":
		return gallery.VisibilityPrivate, true
	default:
		return "", false
	}
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

func (server *Server) viewerFingerprintHash(w stdhttp.ResponseWriter, r *stdhttp.Request) string {
	viewerID := server.viewerID(w, r)
	sum := sha256.Sum256([]byte(viewerID))
	return hex.EncodeToString(sum[:])
}

func (server *Server) viewerID(w stdhttp.ResponseWriter, r *stdhttp.Request) string {
	if cookie, err := r.Cookie(viewerCookieName); err == nil {
		value := strings.TrimSpace(cookie.Value)
		if isValidViewerID(value) {
			return value
		}
	}

	value := newViewerID()
	secure := r.TLS != nil || server.cfg.App.Env == "production"
	stdhttp.SetCookie(w, &stdhttp.Cookie{
		Name:     viewerCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   viewerCookieMaxAge,
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   secure,
	})
	return value
}

func newViewerID() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		fallback := sha256.Sum256([]byte(strings.Join([]string{time.Now().String(), strconv.FormatInt(time.Now().UnixNano(), 10)}, "|")))
		return base64.RawURLEncoding.EncodeToString(fallback[:])
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func isValidViewerID(value string) bool {
	if len(value) < 32 || len(value) > 96 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func clientIPFromRequest(r *stdhttp.Request) string {
	ip := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return ip
}
