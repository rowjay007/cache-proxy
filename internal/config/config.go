package config

import (
	"cache-proxy/internal/errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Server configuration
	Port    int    `json:"port"`
	Host    string `json:"host"`
	Origin  string `json:"origin"`
	Timeout time.Duration `json:"timeout"`
	
	// Cache configuration
	CacheSize     int           `json:"cache_size"`
	CacheTTL      time.Duration `json:"cache_ttl"`
	ClearCache    bool          `json:"clear_cache"`
	
	// Logging configuration
	LogLevel      string `json:"log_level"`
	LogFormat     string `json:"log_format"`
	
	// Security configuration
	EnableCORS    bool     `json:"enable_cors"`
	AllowedOrigins []string `json:"allowed_origins"`
	
	// Health check configuration
	EnableHealthCheck bool `json:"enable_health_check"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Host:              "0.0.0.0",
		Timeout:           30 * time.Second,
		CacheSize:         1000,
		CacheTTL:          5 * time.Minute,
		LogLevel:          "info",
		LogFormat:         "json",
		EnableCORS:        true,
		AllowedOrigins:    []string{"*"},
		EnableHealthCheck: true,
	}
}

// ParseFlags parses command line flags and environment variables
func ParseFlags() (*Config, error) {
	config := DefaultConfig()
	
	var (
		port              = flag.Int("port", getEnvInt("PROXY_PORT", 0), "Port number where proxy runs")
		host              = flag.String("host", getEnvString("PROXY_HOST", config.Host), "Host to bind the server")
		origin            = flag.String("origin", getEnvString("PROXY_ORIGIN", ""), "Origin server to forward requests")
		timeout           = flag.Duration("timeout", getEnvDuration("PROXY_TIMEOUT", config.Timeout), "Request timeout")
		cacheSize         = flag.Int("cache-size", getEnvInt("PROXY_CACHE_SIZE", config.CacheSize), "Maximum number of cache entries")
		cacheTTL          = flag.Duration("cache-ttl", getEnvDuration("PROXY_CACHE_TTL", config.CacheTTL), "Cache time-to-live")
		clearCache        = flag.Bool("clear-cache", false, "Clear cache and exit")
		logLevel          = flag.String("log-level", getEnvString("PROXY_LOG_LEVEL", config.LogLevel), "Log level (debug, info, warn, error)")
		logFormat         = flag.String("log-format", getEnvString("PROXY_LOG_FORMAT", config.LogFormat), "Log format (json, text)")
		enableCORS        = flag.Bool("enable-cors", getEnvBool("PROXY_ENABLE_CORS", config.EnableCORS), "Enable CORS headers")
		allowedOrigins    = flag.String("allowed-origins", getEnvString("PROXY_ALLOWED_ORIGINS", strings.Join(config.AllowedOrigins, ",")), "Comma-separated list of allowed origins")
		enableHealthCheck = flag.Bool("enable-health-check", getEnvBool("PROXY_ENABLE_HEALTH_CHECK", config.EnableHealthCheck), "Enable health check endpoint")
	)
	
	flag.Parse()

	config.Port = *port
	config.Host = *host
	config.Origin = *origin
	config.Timeout = *timeout
	config.CacheSize = *cacheSize
	config.CacheTTL = *cacheTTL
	config.ClearCache = *clearCache
	config.LogLevel = *logLevel
	config.LogFormat = *logFormat
	config.EnableCORS = *enableCORS
	config.EnableHealthCheck = *enableHealthCheck
	
	if *allowedOrigins != "" {
		config.AllowedOrigins = strings.Split(*allowedOrigins, ",")
	}

	return config, config.Validate()
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ClearCache {
		return nil // No validation needed for clear cache command
	}

	if c.Port <= 0 || c.Port > 65535 {
		return errors.Wrap(nil, errors.ErrorTypeValidation, "INVALID_PORT", "port must be between 1 and 65535", 400)
	}

	if c.Origin == "" {
		return errors.Wrap(nil, errors.ErrorTypeValidation, "MISSING_ORIGIN", "origin server URL is required", 400)
	}

	if _, err := url.Parse(c.Origin); err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "INVALID_ORIGIN_URL", "origin server URL is invalid", 400)
	}

	if c.Timeout <= 0 {
		return errors.Wrap(nil, errors.ErrorTypeValidation, "INVALID_TIMEOUT", "timeout must be positive", 400)
	}

	if c.CacheSize <= 0 {
		return errors.Wrap(nil, errors.ErrorTypeValidation, "INVALID_CACHE_SIZE", "cache size must be positive", 400)
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		return errors.Wrap(nil, errors.ErrorTypeValidation, "INVALID_LOG_LEVEL", "log level must be one of: debug, info, warn, error", 400)
	}

	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[c.LogFormat] {
		return errors.Wrap(nil, errors.ErrorTypeValidation, "INVALID_LOG_FORMAT", "log format must be json or text", 400)
	}

	return nil
}

// PrintUsage prints usage information
func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Usage: caching-proxy [options]\n\n")
	fmt.Fprintf(os.Stderr, "Configuration can be provided via command line flags or environment variables.\n")
	fmt.Fprintf(os.Stderr, "Environment variables are prefixed with PROXY_ (e.g., PROXY_PORT).\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}

// Helper functions for environment variable parsing
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
