package http

import (
	stdhttp "net/http"
	"sync"
	"time"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/config"
	"fluffcatch/internal/events"
	"fluffcatch/internal/gallery"
	"fluffcatch/internal/settings"
	"fluffcatch/internal/storage"
	"fluffcatch/internal/uploads"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	cfg             config.Config
	db              *gorm.DB
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

func NewServer(cfg config.Config, dbConn *gorm.DB, storageManager *storage.Manager, settingsService *settings.Service) *Server {
	return &Server{
		cfg:             cfg,
		db:              dbConn,
		storageManager:  storageManager,
		settingsService: settingsService,
		authService:     auth.NewService(dbConn, cfg.Auth.AdminUsername),
		captchaStore:    auth.NewCaptchaStore(),
		eventService:    events.NewService(dbConn, storageManager),
		uploadService:   uploads.NewServiceWithLimits(dbConn, storageManager, cfg.Upload.MaxSizeMB, cfg.Upload.MaxVideoSizeMB),
		galleryService:  gallery.NewService(dbConn, storageManager),
		loginLimiter:    newRateLimiter(1, 5),
		captchaLimiter:  newRateLimiter(2, 10),
		blurCache:       newBlurPreviewCache(),
	}
}

func (server *Server) Routes() stdhttp.Handler {
	if config.IsReleaseEnv(server.cfg.App.Env) {
		gin.SetMode(gin.ReleaseMode)
	} else if server.cfg.App.Env == "test" {
		gin.SetMode(gin.TestMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	if !config.IsReleaseEnv(server.cfg.App.Env) {
		r.Use(gin.Logger())
	}
	server.mountAPIRoutes(r)
	server.mountLocalMedia(r)
	server.mountStaticApp(r)

	return r
}

func (server *Server) mountAPIRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")
	api.GET("/health", server.ginHandler(server.health))

	authRouter := api.Group("/auth")
	authRouter.GET("/captcha", server.ginHandler(server.captcha))
	authRouter.POST("/login", server.ginHandler(server.login))
	authRouter.POST("/logout", server.ginHandler(server.logout))
	authRouter.GET("/me", server.ginHandler(server.me))

	api.GET("/events", server.ginHandler(server.listPublicEvents))
	api.GET("/site", server.ginHandler(server.publicSite))
	api.GET("/events/:id", server.ginHandler(server.getPublicEvent))
	api.GET("/events/:id/photos", server.ginHandler(server.listPublicPhotos))
	api.POST("/events/:id/private-access", server.ginHandler(server.unlockEventPrivatePhotos))
	api.POST("/events/:id/submissions", server.ginHandler(server.createSubmission))
	api.POST("/photos/:id/like", server.ginHandler(server.likePhoto))

	admin := api.Group("/admin")
	admin.Use(server.requireAdminGin)
	admin.GET("/dashboard", server.ginHandler(server.adminDashboard))
	admin.GET("/events", server.ginHandler(server.listAdminEvents))
	admin.POST("/events", server.ginHandler(server.createAdminEvent))
	admin.PUT("/events/:id", server.ginHandler(server.updateAdminEvent))
	admin.DELETE("/events/:id", server.ginHandler(server.deleteAdminEvent))
	admin.POST("/events/:id/cover", server.ginHandler(server.uploadEventCover))
	admin.GET("/events/:id/photos", server.ginHandler(server.listAdminPhotos))
	admin.GET("/events/:id/submissions", server.ginHandler(server.listEventPendingSubmissions))
	admin.GET("/submissions", server.ginHandler(server.listPendingSubmissions))
	admin.POST("/submissions/batch-approve", server.ginHandler(server.approveSubmissions))
	admin.POST("/submissions/batch-delete", server.ginHandler(server.deleteSubmissions))
	admin.PUT("/photos/:id", server.ginHandler(server.updatePhoto))
	admin.DELETE("/photos/:id", server.ginHandler(server.deletePhoto))
	admin.POST("/photos/batch-delete", server.ginHandler(server.batchDeletePhotos))
	admin.POST("/photos/batch-update", server.ginHandler(server.batchUpdatePhotos))
	admin.POST("/change-password", server.ginHandler(server.changePassword))
	admin.GET("/settings", server.ginHandler(server.getSettings))
	admin.PUT("/settings/storage", server.ginHandler(server.updateStorageSettings))
	admin.POST("/settings/storage/test", server.ginHandler(server.testStorageConnection))
	admin.PUT("/settings/oidc", server.ginHandler(server.updateOIDCSettings))
	admin.PUT("/settings/site", server.ginHandler(server.updateSiteSettings))
	admin.POST("/settings/site/logo", server.ginHandler(server.uploadSiteLogo))
	admin.DELETE("/settings/site/logo", server.ginHandler(server.clearSiteLogo))
	admin.POST("/settings/site/background/:variant", server.ginHandler(server.uploadSiteBackground))
	admin.DELETE("/settings/site/background/:variant", server.ginHandler(server.clearSiteBackground))
	admin.PUT("/settings/upload", server.ginHandler(server.updateUploadSettings))
}
