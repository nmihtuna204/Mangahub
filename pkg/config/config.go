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
	"fmt"
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
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	Mode         string        `mapstructure:"mode"` // debug, release
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

// Load reads configuration from file
func Load(configPath string) (*Config, error) {
	viper.SetConfigName("development")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Config file not found, using defaults")
		} else {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	// Allow environment variable override
	viper.AutomaticEnv()

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
}
