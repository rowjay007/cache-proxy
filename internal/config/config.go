package config

import (
	"flag"
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	Port       int
	Origin     string
	ClearCache bool
}

// ParseFlags parses command line flags and returns configuration
func ParseFlags() (*Config, error) {
	var (
		port       = flag.Int("port", 0, "Port number where proxy runs")
		origin     = flag.String("origin", "", "Origin server to forward requests")
		clearCache = flag.Bool("clear-cache", false, "Clear cache and exit")
	)
	flag.Parse()

	config := &Config{
		Port:       *port,
		Origin:     *origin,
		ClearCache: *clearCache,
	}

	return config, config.Validate()
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ClearCache {
		return nil // No validation needed for clear cache command
	}

	if c.Port == 0 {
		return fmt.Errorf("--port is required")
	}

	if c.Origin == "" {
		return fmt.Errorf("--origin is required")
	}

	return nil
}

// PrintUsage prints usage information
func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Usage: caching-proxy [options]\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}
