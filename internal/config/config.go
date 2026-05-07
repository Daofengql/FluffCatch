package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

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
	MaxSizeMB         int `yaml:"max_size_mb"`
	MaxFilesPerUpload int `yaml:"max_files_per_upload"`
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

func (database DatabaseConfig) DSN() (string, error) {
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
	AdminUsername string `yaml:"admin_username"`
	SessionSecret string `yaml:"session_secret"`
}

type OIDCConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Provider     string `yaml:"provider"`
	IssuerURL    string `yaml:"issuer_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
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
			Host:              "127.0.0.1",
			Port:              3306,
			User:              "fluffcatch",
			Password:          "fluffcatch",
			Database:          "fluffcatch",
			Charset:           "utf8mb4",
			Location:          "Local",
			ParseTime:         true,
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
			SessionSecret: "change-me-in-production",
		},
		Frontend: FrontendConfig{
			Mode:       "auto",
			StaticRoot: "www/dist",
		},
		Upload: UploadConfig{
			MaxSizeMB:         20,
			MaxFilesPerUpload: 20,
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
	cfg.Auth.SessionSecret = getEnv("SESSION_SECRET", cfg.Auth.SessionSecret)

	cfg.OIDC.Enabled = getBoolEnv("OIDC_ENABLED", cfg.OIDC.Enabled)
	cfg.OIDC.Provider = getEnv("OIDC_PROVIDER", cfg.OIDC.Provider)
	cfg.OIDC.IssuerURL = getEnv("OIDC_ISSUER_URL", cfg.OIDC.IssuerURL)
	cfg.OIDC.ClientID = getEnv("OIDC_CLIENT_ID", cfg.OIDC.ClientID)
	cfg.OIDC.ClientSecret = getEnv("OIDC_CLIENT_SECRET", cfg.OIDC.ClientSecret)
	cfg.OIDC.RedirectURL = getEnv("OIDC_REDIRECT_URL", cfg.OIDC.RedirectURL)

	cfg.Frontend.Mode = getEnv("FRONTEND_MODE", cfg.Frontend.Mode)
	cfg.Frontend.StaticRoot = getEnv("FRONTEND_STATIC_ROOT", getEnv("STATIC_ROOT", cfg.Frontend.StaticRoot))
	cfg.Upload.MaxSizeMB = getIntEnv("UPLOAD_MAX_SIZE_MB", cfg.Upload.MaxSizeMB)
	cfg.Upload.MaxFilesPerUpload = getIntEnv("UPLOAD_MAX_FILES_PER_UPLOAD", cfg.Upload.MaxFilesPerUpload)
}

func normalizeAndValidate(cfg Config) (Config, error) {
	if cfg.App.Env == "production" && cfg.Auth.SessionSecret == "change-me-in-production" {
		return Config{}, fmt.Errorf("auth.session_secret must be set in production")
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
