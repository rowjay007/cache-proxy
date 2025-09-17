package cache

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Entry represents a cached response with TTL support
type Entry struct {
	Body      []byte        `json:"body"`
	Headers   http.Header   `json:"headers"`
	Status    int           `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	TTL       time.Duration `json:"ttl"`
}

// IsExpired checks if the cache entry has expired
func (e *Entry) IsExpired() bool {
	if e.TTL == 0 {
		return false // No expiration
	}
	return time.Since(e.CreatedAt) > e.TTL
}

// Cache interface defines cache operations
type Cache interface {
	Get(key string) (*Entry, bool)
	Set(key string, entry *Entry) error
	Delete(key string) error
	Clear() error
	Size() int
	Stats() Stats
	GenerateKey(method, path, query string) string
}

// Stats holds cache statistics
type Stats struct {
	Hits        int64 `json:"hits"`
	Misses      int64 `json:"misses"`
	Size        int   `json:"size"`
	Evictions   int64 `json:"evictions"`
	LastCleared time.Time `json:"last_cleared"`
}

// InMemoryCache implements Cache interface with thread-safe operations and TTL support
type InMemoryCache struct {
	data      map[string]*Entry
	mutex     sync.RWMutex
	maxSize   int
	stats     Stats
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// Config holds cache configuration
type Config struct {
	MaxSize       int           `json:"max_size"`
	DefaultTTL    time.Duration `json:"default_ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// New creates a new cache instance with configuration
func New(config Config) Cache {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	cache := &InMemoryCache{
		data:        make(map[string]*Entry),
		maxSize:     config.MaxSize,
		stats:       Stats{LastCleared: time.Now()},
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup goroutine for expired entries
	cache.cleanupTicker = time.NewTicker(config.CleanupInterval)
	go cache.cleanupExpired()

	return cache
}

// Get retrieves a cache entry if it exists and is not expired
func (c *InMemoryCache) Get(key string) (*Entry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	entry, exists := c.data[key]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	if entry.IsExpired() {
		c.mutex.RUnlock()
		c.mutex.Lock()
		delete(c.data, key)
		c.mutex.Unlock()
		c.mutex.RLock()
		c.stats.Misses++
		c.stats.Evictions++
		return nil, false
	}

	c.stats.Hits++
	return entry, true
}

// Set stores a cache entry with eviction if cache is full
func (c *InMemoryCache) Set(key string, entry *Entry) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If cache is full, evict oldest entry
	if len(c.data) >= c.maxSize {
		c.evictOldest()
	}

	entry.CreatedAt = time.Now()
	c.data[key] = entry
	return nil
}

// Delete removes a specific cache entry
func (c *InMemoryCache) Delete(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if _, exists := c.data[key]; exists {
		delete(c.data, key)
		return nil
	}
	return fmt.Errorf("key not found: %s", key)
}

// Clear removes all cache entries
func (c *InMemoryCache) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.data = make(map[string]*Entry)
	c.stats.LastCleared = time.Now()
	return nil
}

// Size returns the current number of cached entries
func (c *InMemoryCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.data)
}

// Stats returns cache statistics
func (c *InMemoryCache) Stats() Stats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	stats := c.stats
	stats.Size = len(c.data)
	return stats
}

// GenerateKey creates a cache key from method, path, and query parameters
func (c *InMemoryCache) GenerateKey(method, path, query string) string {
	content := fmt.Sprintf("%s:%s:%s", method, path, query)
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// evictOldest removes the oldest entry from the cache
func (c *InMemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, entry := range c.data {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}
	
	if oldestKey != "" {
		delete(c.data, oldestKey)
		c.stats.Evictions++
	}
}

// cleanupExpired removes expired entries periodically
func (c *InMemoryCache) cleanupExpired() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.mutex.Lock()
			for key, entry := range c.data {
				if entry.IsExpired() {
					delete(c.data, key)
					c.stats.Evictions++
				}
			}
			c.mutex.Unlock()
		case <-c.stopCleanup:
			c.cleanupTicker.Stop()
			return
		}
	}
}

// Close stops the cleanup goroutine
func (c *InMemoryCache) Close() {
	close(c.stopCleanup)
}
