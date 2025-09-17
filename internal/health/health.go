package health

import (
	"cache-proxy/internal/cache"
	"cache-proxy/internal/logger"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthCheck represents the health status of the application
type HealthCheck struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Uptime    time.Duration     `json:"uptime"`
	Cache     CacheHealthStatus `json:"cache"`
	System    SystemHealthStatus `json:"system"`
}

// CacheHealthStatus represents cache health information
type CacheHealthStatus struct {
	Status string      `json:"status"`
	Stats  cache.Stats `json:"stats"`
}

// SystemHealthStatus represents system health information
type SystemHealthStatus struct {
	Status       string `json:"status"`
	MemoryUsage  string `json:"memory_usage"`
	Goroutines   int    `json:"goroutines"`
}

// Service provides health check functionality
type Service struct {
	cache     cache.Cache
	logger    logger.Logger
	startTime time.Time
	version   string
}

// NewService creates a new health check service
func NewService(cache cache.Cache, logger logger.Logger, version string) *Service {
	return &Service{
		cache:     cache,
		logger:    logger,
		startTime: time.Now(),
		version:   version,
	}
}

// HandleHealthCheck returns a health check handler
func (s *Service) HandleHealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		health := s.GetHealthStatus()
		
		status := http.StatusOK
		if health.Status != "healthy" {
			status = http.StatusServiceUnavailable
		}
		
		c.JSON(status, health)
	}
}

// HandleReadiness returns a readiness check handler
func (s *Service) HandleReadiness() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple readiness check - cache should be available
		if s.cache == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"reason": "cache not available",
			})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"timestamp": time.Now(),
		})
	}
}

// HandleLiveness returns a liveness check handler
func (s *Service) HandleLiveness() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "alive",
			"timestamp": time.Now(),
			"uptime": time.Since(s.startTime),
		})
	}
}

// GetHealthStatus returns the current health status
func (s *Service) GetHealthStatus() HealthCheck {
	cacheStatus := "healthy"
	if s.cache == nil {
		cacheStatus = "unhealthy"
	}
	
	systemStatus := "healthy"
	
	overallStatus := "healthy"
	if cacheStatus != "healthy" || systemStatus != "healthy" {
		overallStatus = "unhealthy"
	}
	
	var cacheStats cache.Stats
	if s.cache != nil {
		cacheStats = s.cache.Stats()
	}
	
	return HealthCheck{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   s.version,
		Uptime:    time.Since(s.startTime),
		Cache: CacheHealthStatus{
			Status: cacheStatus,
			Stats:  cacheStats,
		},
		System: SystemHealthStatus{
			Status: systemStatus,
			MemoryUsage: "N/A", // Could implement actual memory monitoring
			Goroutines: 0,      // Could implement actual goroutine counting
		},
	}
}
