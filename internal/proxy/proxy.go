package proxy

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"cache-proxy/internal/cache"
	"cache-proxy/internal/config"
	"cache-proxy/internal/errors"
	"cache-proxy/internal/health"
	"cache-proxy/internal/logger"
	"cache-proxy/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Server represents the caching proxy server with enterprise features
type Server struct {
	cache         cache.Cache
	originURL     *url.URL
	router        *gin.Engine
	logger        logger.Logger
	config        *config.Config
	healthService *health.Service
	httpServer    *http.Server
	client        *http.Client
}

// New creates a new proxy server instance with enterprise configuration
func New(cfg *config.Config, cacheInstance cache.Cache, log logger.Logger) (*Server, error) {
	parsedURL, err := url.Parse(cfg.Origin)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeValidation, "INVALID_ORIGIN_URL", "failed to parse origin URL", http.StatusBadRequest)
	}

	// Set Gin mode based on log level
	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add enterprise middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.LoggerMiddleware(log))
	router.Use(middleware.SecurityHeaders())
	router.Use(gin.Recovery())
	
	if cfg.EnableCORS {
		router.Use(middleware.CORS())
	}
	
	router.Use(middleware.MetricsMiddleware())

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: false,
		},
	}

	// Create health service
	healthService := health.NewService(cacheInstance, log, "1.0.0")

	server := &Server{
		cache:         cacheInstance,
		originURL:     parsedURL,
		router:        router,
		logger:        log,
		config:        cfg,
		healthService: healthService,
		client:        client,
	}

	// Register routes
	server.registerRoutes()

	log.Info().
		Str("origin", cfg.Origin).
		Int("port", cfg.Port).
		Str("host", cfg.Host).
		Msg("Created new proxy server")

	return server, nil
}

// registerRoutes sets up all server routes
func (s *Server) registerRoutes() {
	// Health check endpoints
	if s.config.EnableHealthCheck {
		s.router.GET("/health", s.healthService.HandleHealthCheck())
		s.router.GET("/health/ready", s.healthService.HandleReadiness())
		s.router.GET("/health/live", s.healthService.HandleLiveness())
	}

	// Cache management endpoints
	s.router.GET("/cache/stats", s.handleCacheStats())
	s.router.DELETE("/cache", s.handleCacheClear())

	// Proxy all other requests
	s.router.Any("/*path", s.handleProxy)
}

// handleCacheStats returns cache statistics
func (s *Server) handleCacheStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := s.cache.Stats()
		c.JSON(http.StatusOK, gin.H{
			"cache_stats": stats,
			"timestamp": time.Now(),
		})
	}
}

// handleCacheClear clears the cache
func (s *Server) handleCacheClear() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := s.cache.Clear(); err != nil {
			s.logger.Error().Err(err).Msg("Failed to clear cache")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cache"})
			return
		}
		
		s.logger.Info().Msg("Cache cleared via API")
		c.JSON(http.StatusOK, gin.H{
			"message": "Cache cleared successfully",
			"timestamp": time.Now(),
		})
	}
}

// handleProxy handles all incoming requests and implements caching logic
func (s *Server) handleProxy(c *gin.Context) {
	cacheKey := s.cache.GenerateKey(c.Request.Method, c.Request.URL.Path, c.Request.URL.RawQuery)

	s.logger.Debug().
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Str("cache_key", cacheKey).
		Str("request_id", c.GetString("request_id")).
		Msg("Processing request")

	if entry, exists := s.cache.Get(cacheKey); exists {
		s.logger.Info().
			Str("cache_key", cacheKey).
			Str("request_id", c.GetString("request_id")).
			Msg("Cache hit")
		s.serveFromCache(c, entry)
		return
	}

	s.logger.Info().
		Str("cache_key", cacheKey).
		Str("request_id", c.GetString("request_id")).
		Msg("Cache miss - forwarding to origin")
	s.forwardToOrigin(c, cacheKey)
}

// serveFromCache serves response from cache
func (s *Server) serveFromCache(c *gin.Context, entry *cache.Entry) {
	// Copy headers from cache
	for key, values := range entry.Headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Header("X-Cache", "HIT")
	c.Data(entry.Status, entry.Headers.Get("Content-Type"), entry.Body)
}

// forwardToOrigin forwards request to origin server and caches response
func (s *Server) forwardToOrigin(c *gin.Context, cacheKey string) {
	ctx := context.WithValue(c.Request.Context(), "request_id", c.GetString("request_id"))
	
	originURL := *s.originURL
	originURL.Path = c.Request.URL.Path
	originURL.RawQuery = c.Request.URL.RawQuery

	req, err := http.NewRequestWithContext(ctx, c.Request.Method, originURL.String(), c.Request.Body)
	if err != nil {
		appErr := errors.Wrap(err, errors.ErrorTypeInternal, "REQUEST_CREATION_FAILED", "Failed to create request to origin server", http.StatusInternalServerError)
		s.logger.Error().Err(appErr).Msg("Request creation failed")
		c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message, "code": appErr.Code})
		return
	}

	// Copy headers from original request
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Make request to origin server using configured client with timeout
	resp, err := s.client.Do(req)
	if err != nil {
		appErr := errors.Wrap(err, errors.ErrorTypeNetwork, "ORIGIN_REQUEST_FAILED", "Failed to reach origin server", http.StatusBadGateway)
		s.logger.Error().Err(appErr).Str("origin", s.originURL.String()).Msg("Origin request failed")
		c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message, "code": appErr.Code})
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		appErr := errors.Wrap(err, errors.ErrorTypeNetwork, "ORIGIN_RESPONSE_READ_FAILED", "Failed to read response from origin server", http.StatusInternalServerError)
		s.logger.Error().Err(appErr).Msg("Failed to read origin response")
		c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message, "code": appErr.Code})
		return
	}

	// Create cache entry with TTL
	entry := &cache.Entry{
		Body:    body,
		Headers: make(http.Header),
		Status:  resp.StatusCode,
		TTL:     s.config.CacheTTL,
	}

	// Copy response headers
	for key, values := range resp.Header {
		entry.Headers[key] = values
	}

	// Store in cache
	if err := s.cache.Set(cacheKey, entry); err != nil {
		s.logger.Error().Err(err).Str("cache_key", cacheKey).Msg("Failed to store entry in cache")
	}

	// Send response to client
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Header("X-Cache", "MISS")
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// Start starts the proxy server with graceful shutdown support
func (s *Server) Start() error {
	addr := s.config.Host + ":" + strconv.Itoa(s.config.Port)
	
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.Timeout,
		WriteTimeout: s.config.Timeout,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info().
		Str("addr", addr).
		Dur("timeout", s.config.Timeout).
		Msg("Starting HTTP server")

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info().Msg("Shutting down server")
	
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return err
		}
	}
	
	// Close cache cleanup goroutine if it implements Close()
	if closer, ok := s.cache.(interface{ Close() }); ok {
		closer.Close()
	}
	
	return nil
}

// ClearCache clears all cache entries (legacy method for backwards compatibility)
func (s *Server) ClearCache() error {
	return s.cache.Clear()
}
