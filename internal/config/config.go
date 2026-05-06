package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Database DatabaseConfig
	Storage  StorageConfig
	Auth     AuthConfig
	OIDC     OIDCConfig
	Frontend FrontendConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host              string
	Port              int
	User              string
	Password          string
	Database          string
	Charset           string
	Location          string
	ParseTime         bool
	ConnectOnStart    bool
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   time.Duration
	ConnMaxIdleTime   time.Duration
	Timeout           time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	ConnectRetries    int
	ConnectRetryDelay time.Duration
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
	Driver        string
	LocalPath     string
	PublicPrefix  string
	PublicBaseURL string
	S3            S3Config
}

type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	AccountID string
}

type AuthConfig struct {
	AdminUsername string
	SessionSecret string
}

type OIDCConfig struct {
	Enabled      bool
	Provider     string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type FrontendConfig struct {
	Mode       string
	StaticRoot string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		App: AppConfig{
			Name: "FluffCatch",
			Env:  getEnv("APP_ENV", "development"),
		},
		HTTP: HTTPConfig{
			Addr:         getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:  getDurationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			Host:              getEnv("MYSQL_HOST", "127.0.0.1"),
			Port:              getIntEnv("MYSQL_PORT", 3306),
			User:              getEnv("MYSQL_USER", "fluffcatch"),
			Password:          getEnv("MYSQL_PASSWORD", "fluffcatch"),
			Database:          getEnv("MYSQL_DATABASE", "fluffcatch"),
			Charset:           getEnv("MYSQL_CHARSET", "utf8mb4"),
			Location:          getEnv("MYSQL_LOCATION", "Local"),
			ParseTime:         getBoolEnv("MYSQL_PARSE_TIME", true),
			ConnectOnStart:    getBoolEnv("MYSQL_CONNECT_ON_START", true),
			MaxOpenConns:      getIntEnv("MYSQL_MAX_OPEN_CONNS", 20),
			MaxIdleConns:      getIntEnv("MYSQL_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime:   getDurationEnv("MYSQL_CONN_MAX_LIFETIME", 25*time.Minute),
			ConnMaxIdleTime:   getDurationEnv("MYSQL_CONN_MAX_IDLE_TIME", 5*time.Minute),
			Timeout:           getDurationEnv("MYSQL_TIMEOUT", 5*time.Second),
			ReadTimeout:       getDurationEnv("MYSQL_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:      getDurationEnv("MYSQL_WRITE_TIMEOUT", 30*time.Second),
			ConnectRetries:    getIntEnv("MYSQL_CONNECT_RETRIES", 5),
			ConnectRetryDelay: getDurationEnv("MYSQL_CONNECT_RETRY_DELAY", 2*time.Second),
		},
		Storage: StorageConfig{
			Driver:        getEnv("STORAGE_DRIVER", "local"),
			LocalPath:     getEnv("STORAGE_LOCAL_PATH", "data/uploads"),
			PublicPrefix:  getEnv("STORAGE_PUBLIC_PREFIX", "/media"),
			PublicBaseURL: strings.TrimRight(getEnv("STORAGE_PUBLIC_BASE_URL", ""), "/"),
			S3: S3Config{
				Endpoint:  getEnv("S3_ENDPOINT", ""),
				Bucket:    getEnv("S3_BUCKET", ""),
				Region:    getEnv("S3_REGION", "us-east-1"),
				AccessKey: getEnv("S3_ACCESS_KEY", ""),
				SecretKey: getEnv("S3_SECRET_KEY", ""),
				UseSSL:    getBoolEnv("S3_USE_SSL", false),
			},
		},
		Auth: AuthConfig{
			AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
			SessionSecret: getEnv("SESSION_SECRET", "change-me-in-production"),
		},
		OIDC: OIDCConfig{
			Enabled:      getBoolEnv("OIDC_ENABLED", false),
			Provider:     getEnv("OIDC_PROVIDER", ""),
			IssuerURL:    getEnv("OIDC_ISSUER_URL", ""),
			ClientID:     getEnv("OIDC_CLIENT_ID", ""),
			ClientSecret: getEnv("OIDC_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("OIDC_REDIRECT_URL", ""),
		},
		Frontend: FrontendConfig{
			Mode:       getEnv("FRONTEND_MODE", "auto"),
			StaticRoot: getEnv("FRONTEND_STATIC_ROOT", getEnv("STATIC_ROOT", "www/dist")),
		},
	}

	if cfg.App.Env == "production" && cfg.Auth.SessionSecret == "change-me-in-production" {
		return Config{}, fmt.Errorf("SESSION_SECRET must be set in production")
	}

	cfg.Storage.Driver = strings.ToLower(cfg.Storage.Driver)
	switch cfg.Storage.Driver {
	case "local", "s3", "minio", "aws-s3", "aliyun-oss", "tencent-cos", "cf-r2":
	default:
		return Config{}, fmt.Errorf("unsupported STORAGE_DRIVER %q", cfg.Storage.Driver)
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
		return fmt.Errorf("unsupported FRONTEND_MODE %q", mode)
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid .env line %d", lineNumber)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("invalid .env line %d", lineNumber)
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		os.Setenv(key, trimEnvValue(value))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return nil
}

func trimEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}

	return value
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
