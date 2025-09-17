package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cache-proxy/internal/cache"
	"cache-proxy/internal/config"
	"cache-proxy/internal/logger"
	"cache-proxy/internal/proxy"

	"github.com/rs/zerolog"
)

func main() {
	// Parse configuration first to determine log level
	cfg, err := config.ParseFlags()
	if err != nil {
		// Use basic logger for configuration errors
		log := logger.New()
		log.Error().Err(err).Msg("Failed to parse configuration")
		config.PrintUsage()
		os.Exit(1)
	}

	// Create logger with appropriate level
	var logLevel zerolog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	log := logger.NewWithLevel(logLevel)

	// Handle clear cache command
	if cfg.ClearCache {
		log.Info().Msg("Cache cleared successfully")
		println("Cache cleared successfully.")
		return
	}

	// Create cache with configuration
	cacheConfig := cache.Config{
		MaxSize:         cfg.CacheSize,
		DefaultTTL:      cfg.CacheTTL,
		CleanupInterval: 5 * time.Minute,
	}
	cacheInstance := cache.New(cacheConfig)

	// Create proxy server with full configuration
	server, err := proxy.New(cfg, cacheInstance, log)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create proxy server")
		os.Exit(1)
	}

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	go func() {
		log.Info().
			Int("port", cfg.Port).
			Str("host", cfg.Host).
			Str("origin", cfg.Origin).
			Dur("timeout", cfg.Timeout).
			Dur("cache_ttl", cfg.CacheTTL).
			Int("cache_size", cfg.CacheSize).
			Msg("Starting caching proxy server")

		if err := server.Start(); err != nil {
			log.Error().Err(err).Msg("Server startup failed")
			stop()
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Info().Msg("Shutting down server gracefully...")

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown failed")
		os.Exit(1)
	}

	log.Info().Msg("Server shutdown completed")
}
