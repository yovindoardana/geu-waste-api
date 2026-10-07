package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	AppEnv           string
	AppPort          int
	DBHost           string
	DBPort           int
	DBName           string
	DBUser           string
	DBPassword       string
	DBSSLMode        string
	UploadDir        string
	DBConnectTimeout time.Duration
	ShutdownTimeout  time.Duration
}

// Valid SSL modes for PostgreSQL.
var validSSLModes = map[string]bool{
	"disable":     true,
	"require":     true,
	"verify-ca":   true,
	"verify-full": true,
}

// Valid App environments.
var validAppEnvs = map[string]bool{
	"development": true,
	"test":        true,
	"production":  true,
}

// Load loads configuration from environment variables (and optionally .env file).
func Load() (*Config, error) {
	// Try loading .env if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:           getEnvOrDefault("APP_ENV", "development"),
		AppPort:          getEnvIntOrDefault("APP_PORT", 8080),
		DBHost:           strings.TrimSpace(os.Getenv("DB_HOST")),
		DBPort:           getEnvIntOrDefault("DB_PORT", 5432),
		DBName:           strings.TrimSpace(os.Getenv("DB_NAME")),
		DBUser:           strings.TrimSpace(os.Getenv("DB_USER")),
		DBPassword:       os.Getenv("DB_PASSWORD"),
		DBSSLMode:        getEnvOrDefault("DB_SSLMODE", "disable"),
		UploadDir:        getEnvOrDefault("UPLOAD_DIR", "./uploads/payment-proofs"),
		DBConnectTimeout: getEnvDurationOrDefault("DB_CONNECT_TIMEOUT", 5*time.Second),
		ShutdownTimeout:  getEnvDurationOrDefault("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration values.
func (c *Config) Validate() error {
	if !validAppEnvs[c.AppEnv] {
		return fmt.Errorf("invalid APP_ENV: %q, must be development, test, or production", c.AppEnv)
	}

	if c.AppPort < 1 || c.AppPort > 65535 {
		return fmt.Errorf("invalid APP_PORT: %d, must be between 1 and 65535", c.AppPort)
	}

	if c.DBHost == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	if c.DBPort < 1 || c.DBPort > 65535 {
		return fmt.Errorf("invalid DB_PORT: %d, must be between 1 and 65535", c.DBPort)
	}

	if c.DBName == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	if c.DBUser == "" {
		return fmt.Errorf("DB_USER is required")
	}

	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}

	if !validSSLModes[c.DBSSLMode] {
		return fmt.Errorf("invalid DB_SSLMODE: %q, must be disable, require, verify-ca, or verify-full", c.DBSSLMode)
	}

	if c.UploadDir == "" {
		return fmt.Errorf("UPLOAD_DIR is required")
	}

	if c.DBConnectTimeout <= 0 {
		return fmt.Errorf("DB_CONNECT_TIMEOUT must be positive")
	}

	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}

	return nil
}

// DSN returns the PostgreSQL connection string with properly encoded credentials.
func (c *Config) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   fmt.Sprintf("%s:%d", c.DBHost, c.DBPort),
		Path:   c.DBName,
	}

	q := u.Query()
	q.Set("sslmode", c.DBSSLMode)
	q.Set("connect_timeout", fmt.Sprintf("%d", int(c.DBConnectTimeout.Seconds())))
	u.RawQuery = q.Encode()

	return u.String()
}

// SafeDSN returns DSN with masked password for logging/debugging.
func (c *Config) SafeDSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, "******"),
		Host:   fmt.Sprintf("%s:%d", c.DBHost, c.DBPort),
		Path:   c.DBName,
	}

	q := u.Query()
	q.Set("sslmode", c.DBSSLMode)
	u.RawQuery = q.Encode()

	return u.String()
}

func getEnvOrDefault(key, defaultValue string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultValue
	}
	return val
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	valStr := strings.TrimSpace(os.Getenv(key))
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return -1 // will fail validation
	}
	return val
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	valStr := strings.TrimSpace(os.Getenv(key))
	if valStr == "" {
		return defaultValue
	}
	dur, err := time.ParseDuration(valStr)
	if err != nil {
		return -1 // will fail validation
	}
	return dur
}
