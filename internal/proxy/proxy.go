package proxy

import (
	"cache-proxy/internal/cache"
	"cache-proxy/internal/logger"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Server struct {
	cache     cache.Cache
	originURL *url.URL
	gin       *gin.Engine
	logger    logger.Logger
}

func New(originURL string, cacheInstance cache.Cache, log logger.Logger) (*Server, error) {
	parsedURL, err := url.Parse(originURL)
	if err != nil {
		return nil, fmt.Errorf("invalid origin URL: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	server := &Server{
		cache:     cacheInstance,
		originURL: parsedURL,
		gin:       router,
		logger:    log,
	}

	log.Info().Str("origin", originURL).Msg("Created new proxy server")

	router.Any("/*path", server.handleProxy)

	return server, nil
}

func (s *Server) handleProxy(c *gin.Context) {
	cacheKey := s.cache.GenerateKey(c.Request.Method, c.Request.URL.Path)

	s.logger.Debug().
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Str("cache_key", cacheKey).
		Msg("Processing request")

	if entry, exists := s.cache.Get(cacheKey); exists {
		s.logger.Info().
			Str("cache_key", cacheKey).
			Msg("Cache hit")
		s.serveFromCache(c, entry)
		return
	}

	s.logger.Info().
		Str("cache_key", cacheKey).
		Msg("Cache miss - forwarding to origin")
	s.forwardToOrigin(c, cacheKey)
}

func (s *Server) serveFromCache(c *gin.Context, entry *cache.Entry) {
	for key, values := range entry.Headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Header("X-Cache", "HIT")
	c.Data(entry.Status, entry.Headers.Get("Content-Type"), entry.Body)
}

func (s *Server) forwardToOrigin(c *gin.Context, cacheKey string) {
	originURL := *s.originURL
	originURL.Path = c.Request.URL.Path
	originURL.RawQuery = c.Request.URL.RawQuery

	req, err := http.NewRequest(c.Request.Method, originURL.String(), c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to reach origin server"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	entry := &cache.Entry{
		Body:    body,
		Headers: make(http.Header),
		Status:  resp.StatusCode,
	}

	for key, values := range resp.Header {
		entry.Headers[key] = values
	}

	s.cache.Set(cacheKey, entry)

	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Header("X-Cache", "MISS")
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func (s *Server) Start(port int) error {
	return s.gin.Run(":" + strconv.Itoa(port))
}

func (s *Server) ClearCache() {
	s.cache.Clear()
}
