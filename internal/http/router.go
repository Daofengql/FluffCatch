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
	cfg              config.Config
	configManager    *config.Manager
	db               *gorm.DB
	storageManager   *storage.Manager
	settingsService  *settings.Service
	authService      *auth.Service
	captchaStore     *auth.CaptchaStore
	eventService     *events.Service
	uploadService    *uploads.Service
	galleryService   *gallery.Service
	loginLimiter     *rateLimiter
	captchaLimiter   *rateLimiter
	eventListLimiter *rateLimiter
	uploadLimiter    chan struct{}
	oidcStates       *oidcStateStore
	blurCache        *blurPreviewCache
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

func NewServer(cfg config.Config, dbConn *gorm.DB, storageManager *storage.Manager, settingsService *settings.Service, configManager ...*config.Manager) *Server {
	manager := config.NewManager("", cfg)
	if len(configManager) > 0 && configManager[0] != nil {
		manager = configManager[0]
	}
	return &Server{
		cfg:              cfg,
		configManager:    manager,
		db:               dbConn,
		storageManager:   storageManager,
		settingsService:  settingsService,
		authService:      auth.NewService(dbConn, manager),
		captchaStore:     auth.NewCaptchaStore(),
		eventService:     events.NewService(dbConn, storageManager),
		uploadService:    uploads.NewServiceWithLimits(dbConn, storageManager, cfg.Upload.MaxSizeMB, cfg.Upload.MaxVideoSizeMB),
		galleryService:   gallery.NewService(dbConn, storageManager),
		loginLimiter:     newRateLimiter(1, 5),
		captchaLimiter:   newRateLimiter(2, 10),
		eventListLimiter: newRateLimiter(6, 12),
		uploadLimiter:    make(chan struct{}, hardMaxConcurrentUploads),
		oidcStates:       newOIDCStateStore(),
		blurCache:        newBlurPreviewCache(),
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
	authRouter.GET("/oidc", server.ginHandler(server.publicOIDCSettings))
	authRouter.GET("/oidc/login", server.ginHandler(server.oidcLoginURL))
	authRouter.GET("/oidc/callback", server.ginHandler(server.oidcCallback))

	api.GET("/events", server.ginHandler(server.listPublicEvents))
	api.GET("/site", server.ginHandler(server.publicSite))
	api.GET("/settings/upload", server.ginHandler(server.publicUploadSettings))
	api.GET("/events/:id", server.ginHandler(server.getPublicEvent))
	api.GET("/events/:id/photos", server.ginHandler(server.listPublicPhotos))
	api.POST("/events/:id/private-access", server.ginHandler(server.unlockEventPrivatePhotos))
	api.GET("/events/:id/submission-token", server.ginHandler(server.resolveSubmissionToken))
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
	admin.POST("/events/:id/cover-from-photo", server.ginHandler(server.setEventCoverFromPhoto))
	admin.GET("/events/:id/photos", server.ginHandler(server.listAdminPhotos))
	admin.GET("/events/:id/submissions", server.ginHandler(server.listEventPendingSubmissions))
	admin.GET("/events/:id/submission-links", server.ginHandler(server.listSubmissionLinks))
	admin.POST("/events/:id/submission-links", server.ginHandler(server.createSubmissionLink))
	admin.DELETE("/events/:id/submission-links/:linkID", server.ginHandler(server.revokeSubmissionLink))
	admin.DELETE("/events/:id/submission-links/:linkID/record", server.ginHandler(server.deleteRevokedSubmissionLink))
	admin.GET("/submissions", server.ginHandler(server.listPendingSubmissions))
	admin.POST("/submissions/batch-approve", server.ginHandler(server.approveSubmissions))
	admin.POST("/submissions/batch-delete", server.ginHandler(server.deleteSubmissions))
	admin.PUT("/photos/:id", server.ginHandler(server.updatePhoto))
	admin.DELETE("/photos/:id", server.ginHandler(server.deletePhoto))
	admin.POST("/photos/batch-delete", server.ginHandler(server.batchDeletePhotos))
	admin.POST("/photos/batch-update", server.ginHandler(server.batchUpdatePhotos))
	admin.POST("/change-password", server.ginHandler(server.changePassword))
	admin.GET("/oidc/status", server.ginHandler(server.oidcStatus))
	admin.POST("/oidc/bind", server.ginHandler(server.oidcBindURL))
	admin.DELETE("/oidc/bind", server.ginHandler(server.oidcUnbind))
	admin.GET("/settings", server.ginHandler(server.getSettings))
	admin.PUT("/settings/storage", server.ginHandler(server.updateStorageSettings))
	admin.POST("/settings/storage/test", server.ginHandler(server.testStorageConnection))
	admin.PUT("/settings/site", server.ginHandler(server.updateSiteSettings))
	admin.POST("/settings/site/logo", server.ginHandler(server.uploadSiteLogo))
	admin.DELETE("/settings/site/logo", server.ginHandler(server.clearSiteLogo))
	admin.POST("/settings/site/background/:variant", server.ginHandler(server.uploadSiteBackground))
	admin.DELETE("/settings/site/background/:variant", server.ginHandler(server.clearSiteBackground))
	admin.PUT("/settings/upload", server.ginHandler(server.updateUploadSettings))
	admin.GET("/maintenance/storage/orphans", server.ginHandler(server.scanOrphanStorageObjects))
	admin.GET("/maintenance/storage/missing-thumbnails", server.ginHandler(server.scanMissingThumbnails))
}
