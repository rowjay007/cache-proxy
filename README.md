# 🛠️ Caching Proxy (Go + Gin)

A CLI-based caching proxy server that forwards requests to an origin server, caches the responses, and serves repeated requests from cache instead of always forwarding them.

**Author:** rowjay  
**Project URL:** https://roadmap.sh/projects/caching-server

## 🎯 Objective

This project demonstrates practical concepts in:
- Go CLI development (flag parsing)
- HTTP proxying with Gin framework
- Response caching (in-memory map with concurrency safety)
- Cache invalidation

## ✅ Features

### Core Functionality
- **HTTP Proxy**: Forwards requests to an origin server
- **Response Caching**: Caches responses in memory for faster subsequent requests
- **Cache Headers**: Adds `X-Cache: HIT` or `X-Cache: MISS` headers
- **Thread-Safe**: Uses `sync.RWMutex` for concurrent access
- **Cache Clearing**: Command to clear all cached entries

### CLI Interface
```bash
# Start proxy server
caching-proxy --port <number> --origin <url>

# Clear cache
caching-proxy --clear-cache
```

## 🚀 Installation & Setup

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd cache-proxy
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Build the application:**
   ```bash
   go build -o caching-proxy ./cmd/caching-proxy
   ```

## 📖 Usage

### Start the Proxy Server

```bash
./caching-proxy --port 3000 --origin http://dummyjson.com
```

This starts the caching proxy at `http://localhost:3000`.
- A request to `http://localhost:3000/products` is forwarded to `http://dummyjson.com/products`

### Example Requests

**First Request (Cache Miss):**
```bash
curl -i http://localhost:3000/products
```
Response includes: `X-Cache: MISS`

**Second Request (Cache Hit):**
```bash
curl -i http://localhost:3000/products
```
Response includes: `X-Cache: HIT`

### Clear Cache

```bash
./caching-proxy --clear-cache
```
Output: `Cache cleared successfully.`

## 🏗️ Architecture

### 1. CLI Layer
- Parses arguments using Go's `flag` package
- Handles `--port`, `--origin`, and `--clear-cache` flags

### 2. Caching Layer
- **Cache Key**: `METHOD + ":" + PATH`
- **Storage**: In-memory map storing:
  - Response body (`[]byte`)
  - Response headers (`http.Header`)
  - HTTP status code (`int`)
- **Thread Safety**: `sync.RWMutex` for concurrent read/write operations

### 3. Proxy Handler (Gin)
For each incoming request:
1. Compute cache key from HTTP method and path
2. Check cache map:
   - **Cache Hit**: Return cached response + `X-Cache: HIT`
   - **Cache Miss**: Forward to origin, cache response, return with `X-Cache: MISS`

### 4. Cache Management
- **Clear Command**: Resets the cache map and confirms with success message
- **Memory Management**: In-memory storage (no persistence)

## 🎪 Example Session

```bash
# Terminal 1: Start the proxy
$ ./caching-proxy --port 3000 --origin http://dummyjson.com
Starting caching proxy on port 3000, forwarding to http://dummyjson.com

# Terminal 2: First request (miss)
$ curl -i http://localhost:3000/products/1
HTTP/1.1 200 OK
X-Cache: MISS
Content-Type: application/json
...

# Terminal 2: Second request (hit)
$ curl -i http://localhost:3000/products/1
HTTP/1.1 200 OK
X-Cache: HIT
Content-Type: application/json
...

# Terminal 2: Clear cache
$ ./caching-proxy --clear-cache
Cache cleared successfully.
```

## 🚀 Stretch Goals

### 1. Cache Expiration (TTL)
Add `--ttl <seconds>` flag to auto-expire cache entries after a specified time.

### 2. Selective Invalidation
Add `/clear?path=/products` endpoint to clear specific cache keys without clearing everything.

### 3. Persistent Cache
Replace in-memory cache with Redis or BoltDB for persistence across restarts.

### 4. Metrics & Observability
Add `/metrics` endpoint for Prometheus to track cache hits/misses, response times, and other metrics.

## 🛠️ Development

### Project Structure
```
cache-proxy/
├── cmd/
│   └── caching-proxy/
│       └── main.go          # Application entry point
├── internal/
│   ├── cache/
│   │   └── cache.go         # Cache interface and implementation
│   ├── config/
│   │   └── config.go        # Configuration parsing and validation
│   ├── logger/
│   │   └── logger.go        # Logging abstraction
│   └── proxy/
│       └── proxy.go         # HTTP proxy server logic
├── go.mod                   # Go module dependencies
├── go.sum                   # Dependency checksums
└── README.md                # Project documentation
```

### Key Components

- **`internal/cache`**: Thread-safe cache interface and in-memory implementation
- **`internal/proxy`**: Gin-based HTTP proxy server with caching logic
- **`internal/config`**: CLI flag parsing and configuration validation
- **`internal/logger`**: Structured logging interface for better observability
- **`cmd/caching-proxy`**: Main application entry point following Go standards

### Testing
Run the application and test with various HTTP methods:
```bash
# GET requests
curl -i http://localhost:3000/products
curl -i http://localhost:3000/users

# POST requests
curl -X POST -d '{"test": "data"}' -H "Content-Type: application/json" http://localhost:3000/posts

# Different paths
curl -i http://localhost:3000/products/1
curl -i http://localhost:3000/products/2
```

## 📝 Requirements Checklist

- ✅ CLI with `--port` and `--origin` flags
- ✅ HTTP proxy functionality with Gin framework
- ✅ In-memory caching with thread safety
- ✅ Cache hit/miss headers (`X-Cache`)
- ✅ Cache clearing with `--clear-cache`
- ✅ Proper error handling and validation
- ✅ Clean, extensible design

## 🤝 Contributing

This project follows clean design principles and is built for extensibility. Feel free to contribute by:
1. Adding the stretch goals mentioned above
2. Improving error handling
3. Adding comprehensive tests
4. Enhancing documentation

## 📄 License

This project is part of the roadmap.sh backend engineering challenges.
