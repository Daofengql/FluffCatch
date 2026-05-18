package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"fluffcatch/internal/buildinfo"

	"github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	HTTP     HTTPConfig     `yaml:"http"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Auth     AuthConfig     `yaml:"auth"`
	OIDC     OIDCConfig     `yaml:"oidc"`
	Frontend FrontendConfig `yaml:"frontend"`
	Upload   UploadConfig   `yaml:"upload"`
}

type UploadConfig struct {
	MaxSizeMB            int `yaml:"max_size_mb"`
	MaxVideoSizeMB       int `yaml:"max_video_size_mb"`
	MaxFilesPerUpload    int `yaml:"max_files_per_upload"`
	DefaultPageSize      int `yaml:"default_page_size"`
	MaxConcurrentUploads int `yaml:"max_concurrent_uploads"`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

type HTTPConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	Driver            string        `yaml:"driver"`
	SQLitePath        string        `yaml:"sqlite_path"`
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	User              string        `yaml:"user"`
	Password          string        `yaml:"password"`
	Database          string        `yaml:"database"`
	Charset           string        `yaml:"charset"`
	Location          string        `yaml:"location"`
	ParseTime         bool          `yaml:"parse_time"`
	ConnectOnStart    bool          `yaml:"connect_on_start"`
	MaxOpenConns      int           `yaml:"max_open_conns"`
	MaxIdleConns      int           `yaml:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime   time.Duration `yaml:"conn_max_idle_time"`
	Timeout           time.Duration `yaml:"timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	ConnectRetries    int           `yaml:"connect_retries"`
	ConnectRetryDelay time.Duration `yaml:"connect_retry_delay"`
}

const (
	DatabaseDriverAuto   = "auto"
	DatabaseDriverMySQL  = "mysql"
	DatabaseDriverSQLite = "sqlite"
)

func (database DatabaseConfig) DSN() (string, error) {
	switch database.EffectiveDriver() {
	case DatabaseDriverSQLite:
		return database.SQLiteDSN(), nil
	default:
		return withMySQLDefaults(database).MySQLDSN()
	}
}

func (database DatabaseConfig) EffectiveDriver() string {
	driver := strings.ToLower(strings.TrimSpace(database.Driver))
	if driver == "" || driver == DatabaseDriverAuto {
		if database.hasMySQLConnectionConfig() {
			return DatabaseDriverMySQL
		}
		return DatabaseDriverSQLite
	}
	return driver
}

func (database DatabaseConfig) MySQLDSN() (string, error) {
	location, err := time.LoadLocation(database.Location)
	if err != nil {
		return "", fmt.Errorf("load mysql location: %w", err)
	}

	params := map[string]string{}
	if database.Charset != "" {
		params["charset"] = database.Charset
	}

	cfg := mysql.NewConfig()
	cfg.User = database.User
	cfg.Passwd = database.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(database.Host, strconv.Itoa(database.Port))
	cfg.DBName = database.Database
	cfg.ParseTime = database.ParseTime
	cfg.Loc = location
	cfg.Params = params
	cfg.Timeout = database.Timeout
	cfg.ReadTimeout = database.ReadTimeout
	cfg.WriteTimeout = database.WriteTimeout
	cfg.CheckConnLiveness = true

	return cfg.FormatDSN(), nil
}

func (database DatabaseConfig) SQLiteDSN() string {
	path := strings.TrimSpace(database.SQLitePath)
	if path == "" {
		path = "data/fluffcatch.db"
	}
	return path
}

func (database DatabaseConfig) hasMySQLConnectionConfig() bool {
	return strings.TrimSpace(database.Host) != "" ||
		database.Port != 0 ||
		strings.TrimSpace(database.User) != "" ||
		strings.TrimSpace(database.Password) != "" ||
		strings.TrimSpace(database.Database) != "" ||
		strings.TrimSpace(database.Charset) != "" ||
		strings.TrimSpace(database.Location) != ""
}

type StorageConfig struct {
	Driver        string   `yaml:"driver"`
	LocalPath     string   `yaml:"local_path"`
	PublicPrefix  string   `yaml:"public_prefix"`
	PublicBaseURL string   `yaml:"public_base_url"`
	S3            S3Config `yaml:"s3"`
}

type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
	AccountID string `yaml:"account_id"`
}

type AuthConfig struct {
	AdminUsername     string `yaml:"admin_username"`
	AdminPasswordHash string `yaml:"admin_password_hash"`
	SessionSecret     string `yaml:"session_secret"`
}

type OIDCConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Provider     string `yaml:"provider"`
	IssuerURL    string `yaml:"issuer_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	BoundSubject string `yaml:"bound_subject"`
}

type FrontendConfig struct {
	Mode       string `yaml:"mode"`
	StaticRoot string `yaml:"static_root"`
}

func Load() (Config, error) {
	return LoadFile("config.yaml")
}

func LoadFile(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = "config.yaml"
	}

	cfg := defaultConfig()
	if err := loadYAML(path, &cfg); err != nil {
		return Config{}, err
	}
	applyEnvOverrides(&cfg)

	return normalizeAndValidate(cfg)
}

func defaultConfig() Config {
	return Config{
		App: AppConfig{
			Name: "FluffCatch",
			Env:  "development",
		},
		HTTP: HTTPConfig{
			Addr:         ":8080",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Driver:            DatabaseDriverAuto,
			SQLitePath:        "data/fluffcatch.db",
			ConnectOnStart:    true,
			MaxOpenConns:      20,
			MaxIdleConns:      10,
			ConnMaxLifetime:   25 * time.Minute,
			ConnMaxIdleTime:   5 * time.Minute,
			Timeout:           5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			ConnectRetries:    5,
			ConnectRetryDelay: 2 * time.Second,
		},
		Storage: StorageConfig{
			Driver:        "local",
			LocalPath:     "data/uploads",
			PublicPrefix:  "/media",
			PublicBaseURL: "",
			S3: S3Config{
				Region: "us-east-1",
			},
		},
		Auth: AuthConfig{
			AdminUsername: "admin",
		},
		Frontend: FrontendConfig{
			Mode:       "auto",
			StaticRoot: "www/dist",
		},
		Upload: UploadConfig{
			MaxSizeMB:            20,
			MaxVideoSizeMB:       500,
			MaxFilesPerUpload:    20,
			DefaultPageSize:      24,
			MaxConcurrentUploads: 2,
		},
	}
}

func loadYAML(path string, cfg *Config) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}

	return nil
}

func applyEnvOverrides(cfg *Config) {
	cfg.App.Env = getEnv("APP_ENV", cfg.App.Env)
	cfg.HTTP.Addr = getEnv("HTTP_ADDR", cfg.HTTP.Addr)
	cfg.HTTP.ReadTimeout = getDurationEnv("HTTP_READ_TIMEOUT", cfg.HTTP.ReadTimeout)
	cfg.HTTP.WriteTimeout = getDurationEnv("HTTP_WRITE_TIMEOUT", cfg.HTTP.WriteTimeout)

	cfg.Database.Driver = getEnv("DATABASE_DRIVER", getEnv("DB_DRIVER", cfg.Database.Driver))
	cfg.Database.SQLitePath = getEnv("SQLITE_PATH", getEnv("DATABASE_SQLITE_PATH", cfg.Database.SQLitePath))
	cfg.Database.Host = getEnv("MYSQL_HOST", cfg.Database.Host)
	cfg.Database.Port = getIntEnv("MYSQL_PORT", cfg.Database.Port)
	cfg.Database.User = getEnv("MYSQL_USER", cfg.Database.User)
	cfg.Database.Password = getEnv("MYSQL_PASSWORD", cfg.Database.Password)
	cfg.Database.Database = getEnv("MYSQL_DATABASE", cfg.Database.Database)
	cfg.Database.Charset = getEnv("MYSQL_CHARSET", cfg.Database.Charset)
	cfg.Database.Location = getEnv("MYSQL_LOCATION", cfg.Database.Location)
	cfg.Database.ParseTime = getBoolEnv("MYSQL_PARSE_TIME", cfg.Database.ParseTime)
	cfg.Database.ConnectOnStart = getBoolEnv("MYSQL_CONNECT_ON_START", cfg.Database.ConnectOnStart)
	cfg.Database.MaxOpenConns = getIntEnv("MYSQL_MAX_OPEN_CONNS", cfg.Database.MaxOpenConns)
	cfg.Database.MaxIdleConns = getIntEnv("MYSQL_MAX_IDLE_CONNS", cfg.Database.MaxIdleConns)
	cfg.Database.ConnMaxLifetime = getDurationEnv("MYSQL_CONN_MAX_LIFETIME", cfg.Database.ConnMaxLifetime)
	cfg.Database.ConnMaxIdleTime = getDurationEnv("MYSQL_CONN_MAX_IDLE_TIME", cfg.Database.ConnMaxIdleTime)
	cfg.Database.Timeout = getDurationEnv("MYSQL_TIMEOUT", cfg.Database.Timeout)
	cfg.Database.ReadTimeout = getDurationEnv("MYSQL_READ_TIMEOUT", cfg.Database.ReadTimeout)
	cfg.Database.WriteTimeout = getDurationEnv("MYSQL_WRITE_TIMEOUT", cfg.Database.WriteTimeout)
	cfg.Database.ConnectRetries = getIntEnv("MYSQL_CONNECT_RETRIES", cfg.Database.ConnectRetries)
	cfg.Database.ConnectRetryDelay = getDurationEnv("MYSQL_CONNECT_RETRY_DELAY", cfg.Database.ConnectRetryDelay)

	cfg.Storage.Driver = getEnv("STORAGE_DRIVER", cfg.Storage.Driver)
	cfg.Storage.LocalPath = getEnv("STORAGE_LOCAL_PATH", cfg.Storage.LocalPath)
	cfg.Storage.PublicPrefix = getEnv("STORAGE_PUBLIC_PREFIX", cfg.Storage.PublicPrefix)
	cfg.Storage.PublicBaseURL = getEnv("STORAGE_PUBLIC_BASE_URL", cfg.Storage.PublicBaseURL)
	cfg.Storage.S3.Endpoint = getEnv("S3_ENDPOINT", cfg.Storage.S3.Endpoint)
	cfg.Storage.S3.Bucket = getEnv("S3_BUCKET", cfg.Storage.S3.Bucket)
	cfg.Storage.S3.Region = getEnv("S3_REGION", cfg.Storage.S3.Region)
	cfg.Storage.S3.AccessKey = getEnv("S3_ACCESS_KEY", cfg.Storage.S3.AccessKey)
	cfg.Storage.S3.SecretKey = getEnv("S3_SECRET_KEY", cfg.Storage.S3.SecretKey)
	cfg.Storage.S3.UseSSL = getBoolEnv("S3_USE_SSL", cfg.Storage.S3.UseSSL)
	cfg.Storage.S3.AccountID = getEnv("S3_ACCOUNT_ID", cfg.Storage.S3.AccountID)

	cfg.Auth.AdminUsername = getEnv("ADMIN_USERNAME", cfg.Auth.AdminUsername)
	cfg.Auth.AdminPasswordHash = getEnv("ADMIN_PASSWORD_HASH", cfg.Auth.AdminPasswordHash)
	cfg.Auth.SessionSecret = getEnv("SESSION_SECRET", cfg.Auth.SessionSecret)

	cfg.OIDC.Enabled = getBoolEnv("OIDC_ENABLED", cfg.OIDC.Enabled)
	cfg.OIDC.Provider = getEnv("OIDC_PROVIDER", cfg.OIDC.Provider)
	cfg.OIDC.IssuerURL = getEnv("OIDC_ISSUER_URL", cfg.OIDC.IssuerURL)
	cfg.OIDC.ClientID = getEnv("OIDC_CLIENT_ID", cfg.OIDC.ClientID)
	cfg.OIDC.ClientSecret = getEnv("OIDC_CLIENT_SECRET", cfg.OIDC.ClientSecret)
	cfg.OIDC.BoundSubject = getEnv("OIDC_BOUND_SUBJECT", cfg.OIDC.BoundSubject)

	cfg.Frontend.Mode = getEnv("FRONTEND_MODE", cfg.Frontend.Mode)
	cfg.Frontend.StaticRoot = getEnv("FRONTEND_STATIC_ROOT", getEnv("STATIC_ROOT", cfg.Frontend.StaticRoot))
	cfg.Upload.MaxSizeMB = getIntEnv("UPLOAD_MAX_SIZE_MB", cfg.Upload.MaxSizeMB)
	cfg.Upload.MaxVideoSizeMB = getIntEnv("UPLOAD_MAX_VIDEO_SIZE_MB", cfg.Upload.MaxVideoSizeMB)
	cfg.Upload.MaxFilesPerUpload = getIntEnv("UPLOAD_MAX_FILES_PER_UPLOAD", cfg.Upload.MaxFilesPerUpload)
	cfg.Upload.DefaultPageSize = getIntEnv("UPLOAD_DEFAULT_PAGE_SIZE", cfg.Upload.DefaultPageSize)
	cfg.Upload.MaxConcurrentUploads = getIntEnv("UPLOAD_MAX_CONCURRENT_UPLOADS", cfg.Upload.MaxConcurrentUploads)
}

func normalizeAndValidate(cfg Config) (Config, error) {
	cfg.Auth.AdminUsername = strings.TrimSpace(cfg.Auth.AdminUsername)
	cfg.Auth.AdminPasswordHash = strings.TrimSpace(cfg.Auth.AdminPasswordHash)
	cfg.Auth.SessionSecret = strings.TrimSpace(cfg.Auth.SessionSecret)
	if cfg.Auth.AdminUsername == "" {
		return Config{}, fmt.Errorf("auth.admin_username is required")
	}
	if cfg.Auth.SessionSecret == "change-me-in-production" {
		cfg.Auth.SessionSecret = ""
	}

	cfg.OIDC.Provider = strings.TrimSpace(cfg.OIDC.Provider)
	cfg.OIDC.IssuerURL = strings.TrimSpace(cfg.OIDC.IssuerURL)
	cfg.OIDC.ClientID = strings.TrimSpace(cfg.OIDC.ClientID)
	cfg.OIDC.ClientSecret = strings.TrimSpace(cfg.OIDC.ClientSecret)
	cfg.OIDC.BoundSubject = strings.TrimSpace(cfg.OIDC.BoundSubject)

	cfg.Database.Driver = cfg.Database.EffectiveDriver()
	switch cfg.Database.Driver {
	case DatabaseDriverMySQL:
		cfg.Database = withMySQLDefaults(cfg.Database)
	case DatabaseDriverSQLite:
		cfg.Database.SQLitePath = strings.TrimSpace(cfg.Database.SQLitePath)
		if cfg.Database.SQLitePath == "" {
			cfg.Database.SQLitePath = "data/fluffcatch.db"
		}
	default:
		return Config{}, fmt.Errorf("unsupported database.driver %q", cfg.Database.Driver)
	}

	cfg.Storage.Driver = strings.ToLower(cfg.Storage.Driver)
	cfg.Storage.PublicBaseURL = strings.TrimRight(cfg.Storage.PublicBaseURL, "/")
	switch cfg.Storage.Driver {
	case "local", "s3", "minio", "aws-s3", "aliyun-oss", "tencent-cos", "cf-r2":
	default:
		return Config{}, fmt.Errorf("unsupported storage.driver %q", cfg.Storage.Driver)
	}

	if err := cfg.SetFrontendMode(cfg.Frontend.Mode); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func withMySQLDefaults(database DatabaseConfig) DatabaseConfig {
	if strings.TrimSpace(database.Host) == "" {
		database.Host = "127.0.0.1"
	}
	if database.Port == 0 {
		database.Port = 3306
	}
	if strings.TrimSpace(database.User) == "" {
		database.User = "fluffcatch"
	}
	if strings.TrimSpace(database.Database) == "" {
		database.Database = "fluffcatch"
	}
	if strings.TrimSpace(database.Charset) == "" {
		database.Charset = "utf8mb4"
	}
	if strings.TrimSpace(database.Location) == "" {
		database.Location = "Local"
	}
	database.ParseTime = true
	return database
}

func IsReleaseEnv(env string) bool {
	normalized := strings.ToLower(strings.TrimSpace(env))
	return buildinfo.IsRelease() || normalized == "production" || normalized == "release"
}

func isProductionEnv(env string) bool {
	normalized := strings.ToLower(strings.TrimSpace(env))
	return normalized == "production" || normalized == "release"
}

func (cfg *Config) SetFrontendMode(mode string) error {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		normalized = "auto"
	}

	switch normalized {
	case "auto", "embedded", "disk", "external", "disabled":
		cfg.Frontend.Mode = normalized
		return nil
	default:
		return fmt.Errorf("unsupported frontend.mode %q", mode)
	}
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
