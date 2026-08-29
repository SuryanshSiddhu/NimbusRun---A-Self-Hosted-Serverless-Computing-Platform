package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the NimbusRun platform.
type Config struct {
	Server        ServerConfig    `mapstructure:"server"`
	Database      DatabaseConfig  `mapstructure:"database"`
	Redis         RedisConfig     `mapstructure:"redis"`
	Auth          AuthConfig      `mapstructure:"auth"`
	Docker        DockerConfig    `mapstructure:"docker"`
	Worker        WorkerConfig    `mapstructure:"worker"`
	Build         BuildConfig     `mapstructure:"build"`
	Observability ObsConfig       `mapstructure:"observability"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbnam"`
	SSLMode  string `mapstructure:"sslmode"`
}

type RedisConfig struct {
	Addr     string       `mapstructure:"addr"`
	Password string       `mapstructure:"password"`
	DB       int          `mapstructure:"db"`
	Stream   StreamConfig `mapstructure:"stream"`
}

type StreamConfig struct {
	JobQueue    string `mapstructure:"job_queue"`
	ResultQueue  string `mapstructure:"result_queue"`
	GroupName    string `mapstructure:"group_name"`
	Consumer     string `mapstructure:"consumer"`
}

type AuthConfig struct {
	JWTSecret   string        `mapstructure:"jwt_secret"`
	JWTExpiry   time.Duration `mapstructure:"jwt_expiry"`
	RefreshExpire time.Duration `mapstructure:"refresh_expiry"`
	APIKeyHeader string       `mapstructure:"api_key_header"`
}

type DockerConfig struct {
	Host           string `mapstructure:"host"`
	Registry       string `mapstructure:"registry"`
	DefaultTimeout int    `mapstructure:"default_timeout"`
}

type WorkerConfig struct {
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `mapstructure:"heartbeat_timeout"`
}

type BuildConfig struct {
	WorkersDir string `mapstructure:"workers_dir"`
	Registry   string `mapstructure:"registry"`
}

type ObsConfig struct {
	MetricsPath string `mapstructure:"metrics_path"`
	MetricsPort string `mapstructure:"metrics_port"`
}

// Load reads configuration from environment variables and config files.
func Load() (*Config, error) {
	v := viper.New()

	v.SetEnvPrefix("NIMBUSRUN")
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", "5432")
	v.SetDefault("database.user", "nimbusrun")
	v.SetDefault("database.password", "nimbusrun")
	v.SetDefault("database.dbname", "nimbusrun")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.stream.job_queue", "nimbusrun:jobs")
	v.SetDefault("redis.stream.result_queue", "nimbusrun:results")
	v.SetDefault("redis.stream.group_name", "nimbusrun:workers")
	v.SetDefault("redis.stream.consumer", "worker-1")
	v.SetDefault("auth.jwt_secret", "super-secret-key-change-me")
	v.SetDefault("auth.jwt_expiry", "24h")
	v.SetDefault("auth.refresh_expiry", "168h")
	v.SetDefault("auth.api_key_header", "X-API-Key")
	v.SetDefault("docker.host", "unix:///var/run/docker.sock")
	v.SetDefault("docker.registry", "localhost:5000")
	v.SetDefault("docker.default_timeout", 30)
	v.SetDefault("worker.heartbeat_interval", "5s")
	v.SetDefault("worker.heartbeat_timeout", "15s")
	v.SetDefault("observability.metrics_path", "/metrics")
	v.SetDefault("observability.metrics_port", "9090")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}