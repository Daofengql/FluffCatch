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

	"gorm.io/gorm"
)

type cliOptions struct {
	migrateOnly        bool
	resetAdminPassword bool
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

	if options.frontendMode != "" {
		if err := cfg.SetFrontendMode(options.frontendMode); err != nil {
			slog.Error("invalid frontend mode", "error", err)
			os.Exit(1)
		}
	}

	if options.migrateOnly {
		runMigrationsAndExit(ctx, cfg)
		return
	}

	if options.resetAdminPassword {
		resetAdminPasswordAndExit(ctx, cfg, options.adminPassword)
		return
	}

	gormDB := mustOpenDatabase(ctx, cfg)
	if gormDB != nil {
		sqlDB, err := gormDB.DB()
		if err != nil {
			slog.Error("failed to access mysql handle", "error", err)
			os.Exit(1)
		}
		defer sqlDB.Close()
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

	appServer := apphttp.NewServer(cfg, gormDB, storageManager, settingsService)
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
	flag.StringVar(&options.adminPassword, "admin-password", "", "admin password to set with --reset-admin-password; generated when omitted")
	flag.StringVar(&options.configFile, "config", "config.yaml", "YAML config file to load")
	flag.StringVar(&options.frontendMode, "frontend-mode", "", "frontend serving mode: auto, embedded, disk, external, disabled")
	flag.Parse()
	return options
}

func runMigrationsAndExit(ctx context.Context, cfg config.Config) {
	dsn, err := cfg.Database.DSN()
	if err != nil {
		slog.Error("failed to build mysql dsn", "error", err)
		os.Exit(1)
	}

	gormDB, err := db.OpenGORM(ctx, dsn, databaseOptionsFromConfig(cfg.Database))
	if err != nil {
		slog.Error("failed to connect mysql", "error", err)
		os.Exit(1)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		slog.Error("failed to access mysql handle", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := db.Migrate(ctx, sqlDB); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	password, created, err := auth.EnsureInitialAdmin(ctx, gormDB, cfg.Auth.AdminUsername)
	if err != nil {
		slog.Error("failed to ensure initial admin", "error", err)
		os.Exit(1)
	}

	slog.Info("database migrations completed")
	if created {
		slog.Info("initial admin user created", "username", cfg.Auth.AdminUsername, "password", password)
		slog.Info("save this password now; it will not be shown again")
	} else {
		slog.Info("admin user already exists; no password was generated")
	}
	slog.Info("please restart FluffCatch without --migrate to enter the main system")
}

func resetAdminPasswordAndExit(ctx context.Context, cfg config.Config, password string) {
	dsn, err := cfg.Database.DSN()
	if err != nil {
		slog.Error("failed to build mysql dsn", "error", err)
		os.Exit(1)
	}

	gormDB, err := db.OpenGORM(ctx, dsn, databaseOptionsFromConfig(cfg.Database))
	if err != nil {
		slog.Error("failed to connect mysql", "error", err)
		os.Exit(1)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		slog.Error("failed to access mysql handle", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	password, err = auth.ResetAdminPassword(ctx, gormDB, cfg.Auth.AdminUsername, password)
	if err != nil {
		slog.Error("failed to reset admin password", "error", err)
		os.Exit(1)
	}

	slog.Info("admin password reset completed", "username", cfg.Auth.AdminUsername, "password", password)
	slog.Info("please restart FluffCatch without --reset-admin-password to enter the main system")
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
		slog.Info("mysql connection skipped", "reason", "database.connect_on_start is false")
		return nil
	}

	dsn, err := cfg.Database.DSN()
	if err != nil {
		slog.Error("failed to build mysql dsn", "error", err)
		os.Exit(1)
	}

	dbConn, err := db.OpenGORM(ctx, dsn, databaseOptionsFromConfig(cfg.Database))
	if err != nil {
		slog.Error("failed to connect mysql", "error", err)
		os.Exit(1)
	}

	return dbConn
}

func databaseOptionsFromConfig(database config.DatabaseConfig) db.Options {
	return db.Options{
		MaxOpenConns:      database.MaxOpenConns,
		MaxIdleConns:      database.MaxIdleConns,
		ConnMaxLifetime:   database.ConnMaxLifetime,
		ConnMaxIdleTime:   database.ConnMaxIdleTime,
		ConnectRetries:    database.ConnectRetries,
		ConnectRetryDelay: database.ConnectRetryDelay,
	}
}
