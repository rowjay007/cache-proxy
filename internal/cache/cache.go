package cache

import (
	"net/http"
	"sync"
)

type Entry struct {
	Body    []byte
	Headers http.Header
	Status  int
}

type Cache interface {
	Get(key string) (*Entry, bool)
	Set(key string, entry *Entry)
	Clear()
	GenerateKey(method, path string) string
}

type InMemoryCache struct {
	data  map[string]*Entry
	mutex sync.RWMutex
}

func New() Cache {
	return &InMemoryCache{
		data: make(map[string]*Entry),
	}
}

func (c *InMemoryCache) Get(key string) (*Entry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	entry, exists := c.data[key]
	return entry, exists
}

func (c *InMemoryCache) Set(key string, entry *Entry) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data[key] = entry
}

func (c *InMemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data = make(map[string]*Entry)
}

func (c *InMemoryCache) GenerateKey(method, path string) string {
	return method + ":" + path
}
