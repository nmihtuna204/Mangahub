// Package config - Application Configuration Management
// Xử lý load và parse configuration từ YAML files
// Chức năng:
//   - Load config từ development.yaml/production.yaml
//   - Server, Database, JWT, TCP, UDP, gRPC, WebSocket configs
//   - Logging configuration
//   - Environment-specific settings
//   - Sử dụng Viper cho flexible config loading
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	TCP       TCPConfig
	UDP       UDPConfig
	GRPC      GRPCConfig
	WebSocket WebSocketConfig
	Logging   LoggingConfig
	Redis     RedisConfig
	MangaDex  MangaDexConfig
	Jikan     JikanConfig
	AniList   AniListConfig
	Chapters  ChaptersConfig
}

// ChaptersConfig controls the background check for new chapters
type ChaptersConfig struct {
	SyncInterval time.Duration `mapstructure:"sync_interval"` // 0 disables the background sync
	Language     string        `mapstructure:"language"`      // chapters counted in this translation
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	Mode         string        `mapstructure:"mode"` // debug, release

	// Per client IP; 0 disables. AuthRateLimit (per minute) applies to
	// /auth/login and /auth/register on top of RateLimit.
	RateLimit     float64 `mapstructure:"rate_limit"` // requests per second
	RateBurst     int     `mapstructure:"rate_burst"`
	AuthRateLimit float64 `mapstructure:"auth_rate_limit"` // attempts per minute
	AuthRateBurst int     `mapstructure:"auth_rate_burst"`
}

type DatabaseConfig struct {
	Path            string        `mapstructure:"path"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	Expiration time.Duration `mapstructure:"expiration"`
	Issuer     string        `mapstructure:"issuer"`
}

type TCPConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	MaxConnections int    `mapstructure:"max_connections"`
	BufferSize     int    `mapstructure:"buffer_size"`
}

type UDPConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	BufferSize int    `mapstructure:"buffer_size"`
	// Subscribers that don't re-send REGISTER within this are dropped
	SubscriberTTL time.Duration `mapstructure:"subscriber_ttl"`
}

type GRPCConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type WebSocketConfig struct {
	Host             string        `mapstructure:"host"`
	Port             int           `mapstructure:"port"`
	ReadBufferSize   int           `mapstructure:"read_buffer_size"`
	WriteBufferSize  int           `mapstructure:"write_buffer_size"`
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"`
	PingPeriod       time.Duration `mapstructure:"ping_period"`
	MaxMessageSize   int64         `mapstructure:"max_message_size"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// RedisConfig holds Redis cache configuration
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// MangaDexConfig holds MangaDex API configuration
type MangaDexConfig struct {
	BaseURL       string        `mapstructure:"base_url"`
	RateLimit     int           `mapstructure:"rate_limit"`
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
}

// JikanConfig holds Jikan API configuration
type JikanConfig struct {
	BaseURL       string        `mapstructure:"base_url"`
	RateLimit     int           `mapstructure:"rate_limit"`
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
}

// AniListConfig holds AniList GraphQL API configuration
type AniListConfig struct {
	BaseURL       string        `mapstructure:"base_url"`
	RateLimit     int           `mapstructure:"rate_limit"`
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
}

// Load reads configuration from file.
// The MANGAHUB_CONFIG environment variable, if set, overrides configPath.
// Any key can also be overridden by an environment variable named after it
// with dots replaced by underscores, e.g. TCP_HOST=tcp-server or SERVER_PORT=8081.
func Load(configPath string) (*Config, error) {
	if env := os.Getenv("MANGAHUB_CONFIG"); env != "" {
		configPath = env
	}
	viper.SetConfigType("yaml")
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("development")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
	}

	// Allow environment variable override
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist) {
			fmt.Println("Config file not found, using defaults")
		} else {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "15s")
	viper.SetDefault("server.write_timeout", "15s")
	viper.SetDefault("server.idle_timeout", "60s")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.rate_limit", 50)
	viper.SetDefault("server.rate_burst", 100)
	viper.SetDefault("server.auth_rate_limit", 10)
	viper.SetDefault("server.auth_rate_burst", 10)

	// Database defaults
	viper.SetDefault("database.path", "./data/mangahub.db")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", "5m")

	// JWT defaults
	viper.SetDefault("jwt.secret", "your-secret-key-change-in-production")
	viper.SetDefault("jwt.expiration", "24h")
	viper.SetDefault("jwt.issuer", "mangahub")

	// TCP defaults
	viper.SetDefault("tcp.host", "localhost")
	viper.SetDefault("tcp.port", 9090)
	viper.SetDefault("tcp.max_connections", 100)
	viper.SetDefault("tcp.buffer_size", 4096)

	// UDP defaults
	viper.SetDefault("udp.host", "localhost")
	viper.SetDefault("udp.port", 9091)
	viper.SetDefault("udp.buffer_size", 2048)
	viper.SetDefault("udp.subscriber_ttl", "5m")

	// gRPC defaults
	viper.SetDefault("grpc.host", "localhost")
	viper.SetDefault("grpc.port", 9092)

	// WebSocket defaults
	viper.SetDefault("websocket.host", "localhost")
	viper.SetDefault("websocket.port", 9093)
	viper.SetDefault("websocket.read_buffer_size", 1024)
	viper.SetDefault("websocket.write_buffer_size", 1024)
	viper.SetDefault("websocket.handshake_timeout", "10s")
	viper.SetDefault("websocket.ping_period", "54s")
	viper.SetDefault("websocket.max_message_size", 512000)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// MangaDex API defaults
	viper.SetDefault("mangadex.base_url", "https://api.mangadex.org")
	viper.SetDefault("mangadex.rate_limit", 5)
	viper.SetDefault("mangadex.timeout", "30s")
	viper.SetDefault("mangadex.retry_attempts", 3)

	// Jikan API defaults
	viper.SetDefault("jikan.base_url", "https://api.jikan.moe/v4")
	viper.SetDefault("jikan.rate_limit", 3)
	viper.SetDefault("jikan.timeout", "30s")
	viper.SetDefault("jikan.retry_attempts", 3)

	// New chapter sync (off by default: it calls the MangaDex API)
	viper.SetDefault("chapters.sync_interval", "0s")
	viper.SetDefault("chapters.language", "en")

	// AniList API defaults
	viper.SetDefault("anilist.base_url", "https://graphql.anilist.co")
	viper.SetDefault("anilist.rate_limit", 30)
	viper.SetDefault("anilist.timeout", "30s")
	viper.SetDefault("anilist.retry_attempts", 3)
}
