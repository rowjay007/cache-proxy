package main

import (
	"cache-proxy/internal/cache"
	"cache-proxy/internal/config"
	"cache-proxy/internal/logger"
	"cache-proxy/internal/proxy"
	"fmt"
	"os"
)

func main() {
	log := logger.New()
	
	cfg, err := config.ParseFlags()
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse configuration")
		config.PrintUsage()
		os.Exit(1)
	}

	if cfg.ClearCache {
		log.Info().Msg("Cache cleared successfully")
		fmt.Println("Cache cleared successfully.")
		return
	}

	cacheInstance := cache.New()

	server, err := proxy.New(cfg.Origin, cacheInstance, log)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create proxy server")
		os.Exit(1)
	}

	log.Info().
		Int("port", cfg.Port).
		Str("origin", cfg.Origin).
		Msg("Starting caching proxy server")

	if err := server.Start(cfg.Port); err != nil {
		log.Error().Err(err).Msg("Failed to start server")
		os.Exit(1)
	}
}
