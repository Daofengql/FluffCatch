package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fluffcatch/internal/auth"
	"fluffcatch/internal/config"
	"fluffcatch/internal/db"
	apphttp "fluffcatch/internal/http"
	"fluffcatch/internal/settings"
	"fluffcatch/internal/storage"
	"fluffcatch/internal/tasks"

	"gorm.io/gorm"
)

type cliOptions struct {
	migrateOnly        bool
	resetAdminPassword bool
	backfillEXIF       bool
	adminPassword      string
	frontendMode       string
	configFile         string
}

func main() {
	options := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadFile(options.configFile)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	configManager := config.NewManager(options.configFile, cfg)

	if options.frontendMode != "" {
		if err := cfg.SetFrontendMode(options.frontendMode); err != nil {
			slog.Error("invalid frontend mode", "error", err)
			os.Exit(1)
		}
	}

	if options.migrateOnly {
		runMigrationsAndExit(ctx, cfg, configManager)
		return
	}

	if options.resetAdminPassword {
		resetAdminPasswordAndExit(ctx, cfg, configManager, options.adminPassword)
		return
	}

	if options.backfillEXIF {
		backfillEXIFAndExit(ctx, cfg)
		return
	}

	gormDB := mustOpenDatabase(ctx, cfg)
	if gormDB != nil {
		sqlDB, err := gormDB.DB()
		if err != nil {
			slog.Error("failed to access database handle", "error", err)
			os.Exit(1)
		}
		defer sqlDB.Close()
	}

	if _, created, err := auth.EnsureConfigSessionSecret(ctx, configManager); err != nil {
		slog.Error("failed to ensure session secret config", "error", err)
		os.Exit(1)
	} else if created {
		cfg = configManager.Current()
		slog.Info("session secret written to config")
	}

	if password, created, err := auth.EnsureConfigAdminPassword(ctx, configManager); err != nil {
		slog.Error("failed to ensure admin password config", "error", err)
		os.Exit(1)
	} else if created {
		cfg = configManager.Current()
		slog.Info("initial admin password hash written to config", "username", cfg.Auth.AdminUsername, "password", password)
		slog.Info("save this password now; it will not be shown again")
	}

	settingsService := settings.NewServiceWithReferences(
		settings.NewStore(gormDB, settings.FromConfig(cfg)),
		settings.NewGORMPolicyReferenceChecker(gormDB),
	)
	runtimeSettings, err := settingsService.Load(ctx)
	if err != nil {
		slog.Error("failed to load runtime settings", "error", err)
		os.Exit(1)
	}

	storageManager, err := storage.NewManager(
		runtimeSettings.StoragePolicies.ActivePolicyID,
		storageConfigsFromPolicies(runtimeSettings.StoragePolicies.Policies),
	)
	if err != nil {
		slog.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}

	appServer := apphttp.NewServer(cfg, gormDB, storageManager, settingsService, configManager)
	httpServer := &stdhttp.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      appServer.Routes(),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	go func() {
		slog.Info("fluffcatch listening", "addr", cfg.HTTP.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			slog.Error("http server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func parseFlags() cliOptions {
	var options cliOptions
	flag.BoolVar(&options.migrateOnly, "migrate", false, "run database migrations and exit")
	flag.BoolVar(&options.resetAdminPassword, "reset-admin-password", false, "reset admin password and exit")
	flag.BoolVar(&options.backfillEXIF, "backfill-exif", false, "backfill EXIF metadata for existing uploaded images and exit")
	flag.StringVar(&options.adminPassword, "admin-password", "", "admin password to set with --reset-admin-password; generated when omitted")
	flag.StringVar(&options.configFile, "config", "config.yaml", "YAML config file to load")
	flag.StringVar(&options.frontendMode, "frontend-mode", "", "frontend serving mode: auto, embedded, disk, external, disabled")
	flag.Parse()
	return options
}

func runMigrationsAndExit(ctx context.Context, cfg config.Config, configManager *config.Manager) {
	dsn, err := cfg.Database.DSN()
	if err != nil {
		slog.Error("failed to build database dsn", "driver", cfg.Database.Driver, "error", err)
		os.Exit(1)
	}

	gormDB, err := db.OpenGORM(ctx, dsn, databaseOptionsFromConfig(cfg.Database, cfg.App))
	if err != nil {
		slog.Error("failed to connect database", "driver", cfg.Database.Driver, "error", err)
		os.Exit(1)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		slog.Error("failed to access database handle", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := db.Migrate(ctx, sqlDB, cfg.Database.Driver); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	if _, created, err := auth.EnsureConfigSessionSecret(ctx, configManager); err != nil {
		slog.Error("failed to ensure session secret config", "error", err)
		os.Exit(1)
	} else if created {
		slog.Info("session secret written to config")
	}

	password, created, err := auth.EnsureConfigAdminPassword(ctx, configManager)
	if err != nil {
		slog.Error("failed to ensure admin password config", "error", err)
		os.Exit(1)
	}

	slog.Info("database migrations completed")
	if created {
		cfg = configManager.Current()
		slog.Info("initial admin password hash written to config", "username", cfg.Auth.AdminUsername, "password", password)
		slog.Info("save this password now; it will not be shown again")
	} else {
		slog.Info("admin password hash already exists in config; no password was generated")
	}
	slog.Info("please restart FluffCatch without --migrate to enter the main system")
}

func resetAdminPasswordAndExit(ctx context.Context, cfg config.Config, configManager *config.Manager, password string) {
	password, err := auth.ResetConfigAdminPassword(ctx, configManager, password)
	if err != nil {
		slog.Error("failed to reset admin password", "error", err)
		os.Exit(1)
	}
	cfg = configManager.Current()
	slog.Info("admin password reset completed", "username", cfg.Auth.AdminUsername, "password", password)
	slog.Info("please restart FluffCatch without --reset-admin-password to enter the main system")
}

func backfillEXIFAndExit(ctx context.Context, cfg config.Config) {
	gormDB := mustOpenDatabase(ctx, cfg)
	if gormDB == nil {
		slog.Error("database connection is required for EXIF backfill")
		os.Exit(1)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		slog.Error("failed to access database handle", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	settingsService := settings.NewServiceWithReferences(
		settings.NewStore(gormDB, settings.FromConfig(cfg)),
		settings.NewGORMPolicyReferenceChecker(gormDB),
	)
	runtimeSettings, err := settingsService.Load(ctx)
	if err != nil {
		slog.Error("failed to load runtime settings", "error", err)
		os.Exit(1)
	}
	storageManager, err := storage.NewManager(
		runtimeSettings.StoragePolicies.ActivePolicyID,
		storageConfigsFromPolicies(runtimeSettings.StoragePolicies.Policies),
	)
	if err != nil {
		slog.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}

	result, err := tasks.BackfillEXIF(ctx, gormDB, storageManager)
	if err != nil {
		if tasks.IsContextCanceled(err) {
			slog.Warn("EXIF backfill interrupted", "scanned", result.Scanned, "updated", result.Updated, "failed", result.Failed)
		} else {
			slog.Error("EXIF backfill failed", "error", err)
		}
		os.Exit(1)
	}
	if tasks.IsNoBackfillWork(result) {
		slog.Info("EXIF backfill completed; no candidate media found")
		return
	}
	slog.Info(
		"EXIF backfill completed",
		"scanned", result.Scanned,
		"updated", result.Updated,
		"skipped", result.Skipped,
		"failed", result.Failed,
		"photoRows", result.PhotoRows,
		"submissionRows", result.SubmissionRows,
		"bytesRead", result.BytesRead,
	)
	if result.Failed > 0 {
		os.Exit(1)
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

func mustOpenDatabase(ctx context.Context, cfg config.Config) *gorm.DB {
	if !cfg.Database.ConnectOnStart {
		slog.Info("database connection skipped", "reason", "database.connect_on_start is false")
		return nil
	}

	dsn, err := cfg.Database.DSN()
	if err != nil {
		slog.Error("failed to build database dsn", "driver", cfg.Database.Driver, "error", err)
		os.Exit(1)
	}
	sqliteNeedsInitialMigration := false
	if cfg.Database.Driver == config.DatabaseDriverSQLite {
		needsMigration, err := db.SQLiteNeedsInitialMigration(dsn)
		if err != nil {
			slog.Error("failed to inspect sqlite database", "error", err)
			os.Exit(1)
		}
		sqliteNeedsInitialMigration = needsMigration
	}

	dbConn, err := db.OpenGORM(ctx, dsn, databaseOptionsFromConfig(cfg.Database, cfg.App))
	if err != nil {
		slog.Error("failed to connect database", "driver", cfg.Database.Driver, "error", err)
		os.Exit(1)
	}
	if sqliteNeedsInitialMigration {
		sqlDB, err := dbConn.DB()
		if err != nil {
			slog.Error("failed to access database handle", "error", err)
			os.Exit(1)
		}
		if err := db.Migrate(ctx, sqlDB, cfg.Database.Driver); err != nil {
			slog.Error("sqlite initial migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("sqlite database initialized", "path", cfg.Database.SQLitePath)
	}

	return dbConn
}

func databaseOptionsFromConfig(database config.DatabaseConfig, app config.AppConfig) db.Options {
	return db.Options{
		Driver:            database.Driver,
		MaxOpenConns:      database.MaxOpenConns,
		MaxIdleConns:      database.MaxIdleConns,
		ConnMaxLifetime:   database.ConnMaxLifetime,
		ConnMaxIdleTime:   database.ConnMaxIdleTime,
		ConnectRetries:    database.ConnectRetries,
		ConnectRetryDelay: database.ConnectRetryDelay,
		Quiet:             config.IsReleaseEnv(app.Env),
	}
}
