# Building a Production-Ready Caching Proxy in Go

---

I've seen the same pattern play out time and again: a successful application's biggest enemy is its own growth. As traffic scales, the backend inevitably becomes a bottleneck, leading to slower response times and higher costs. To solve this, I often turn to one of the most powerful tools we have: a caching proxy.

In this post, I'm going to walk you through how I designed and built a production-ready caching proxy in Go. We're going to move beyond a simple 'hello world' example and dive deep into the architectural decisions, trade-offs, and patterns that are required to build a service you can truly rely on in a demanding enterprise environment.

## My Core Principles:

For me, it's a promise that the service delivers on five key principles. These were the non-negotiable requirements I set for this project:

- **Resilience**: The proxy can't just fail; it has to handle upstream failures gracefully and never cause a cascading outage.
- **Observability**: I need to be able to see everything. The service can't be a black box; it must provide deep insights through structured logs, metrics, and traces.
- **Scalability**: It has to be ready for 10x the traffic from day one, designed for clean, horizontal scaling.
- **Security**: Security must be built-in, not bolted on. I designed it to operate on a principle of least privilege.
- **Maintainability**: I wanted to build a codebase that other engineers would enjoy working on—clean, modular, and easy to test.

Everything we're about to explore was built on this foundation.

## The Architecture: A Tour of the Components

I'm a firm believer in the **Separation of Concerns**. I designed this proxy not as a monolith, but as a collection of specialized Go packages that work together. Each one has a single, clear job.

```text
cache-proxy/
├── cmd/caching-proxy/main.go     # The Conductor: Starts and stops the service
├── internal/
│   ├── cache/cache.go            # The Heart: The high-performance cache itself
│   ├── config/config.go          # The Brain: Manages all configuration
│   ├── errors/errors.go          # The Safety Net: Handles errors predictably
│   ├── health/health.go          # The Pulse: Reports the service's health
│   ├── logger/logger.go          # The Scribe: Writes structured logs
│   ├── middleware/middleware.go  # The Guardian: Protects and enriches requests
│   └── proxy/proxy.go            # The Engine Room: The core HTTP server
└── Taskfile.yml                  # The Toolkit: For building and testing
```

I stick to the standard Go project layout because it makes the codebase immediately familiar to any Go developer. Now, let's dive into the components I'm most proud of.

### The Heart: A High-Concurrency Caching Engine

At the core of the proxy is the caching engine. I knew from the start that a simple `map[string][]byte` wouldn't cut it; in a concurrent system, that's a guaranteed race condition. So, I built the engine in `internal/cache/cache.go` specifically for thread safety and high performance.

I started by defining a clean `Cache` interface. This is one of my favorite patterns in Go because it makes the code incredibly modular. It means that later on, we can easily swap in a Redis-backed cache without having to rewrite the entire proxy.

```go

type Cache interface {
    Get(key string) (*Entry, bool)
    Set(key, value string, ttl time.Duration)
    Delete(key string) bool
    Stats() Stats
}
```

My `InMemoryCache` implementation showcases a few pragmatic Go concurrency patterns:

```go

type InMemoryCache struct {
	mu         sync.RWMutex
	items      map[string]*Entry
	maxSize    int
	stats      Stats
	defaultTTL time.Duration
	cleanup    *time.Ticker
}
```

- **I chose a `sync.RWMutex` for locking.** A standard `Mutex` would have created a bottleneck, since cache reads are far more common than writes. The `RWMutex` allows for unlimited concurrent readers, which is a huge performance win.
- **I use a background goroutine for cache cleanup.** A simple `time.Ticker` wakes up a goroutine periodically to purge expired items. This is an elegant, low-overhead way to handle TTLs and prevent stale data.
- **I enforced a `maxSize` to prevent memory leaks.** An unbounded cache is a dangerous thing. My implementation guarantees a predictable memory footprint by evicting the oldest item when the cache is full.
- **The cache tracks its own metrics.** It's not a black box. I made sure it tracks hits, misses, and evictions—vital signs that we can expose through an API for monitoring.

### The Brain: Smart, 12-Factor Configuration

I'm a zealot when it comes to configuration. Hardcoding settings is a non-starter for me, so I strictly followed the [12-Factor App methodology](https://12factor.net/config). All configuration is externalized and managed in a single place: `internal/config/config.go`.

```go

type Config struct {
	Port         int           `json:"port"`
	Origin       string        `json:"origin"`
	CacheTTL     time.Duration `json:"cache_ttl"`
	CacheSize    int           `json:"cache_size"`
	Timeout      time.Duration `json:"timeout"`
	LogLevel     string        `json:"log_level"`
    // ... and more
}

func (c *Config) Validate() error {
    if c.Port <= 0 || c.Port > 65535 {
        return errors.New("invalid port")
    }
    // ... other validations
    return nil
}
```

- **Environment is King**: I designed the service so that a single, immutable Docker image can be promoted from `dev` to `staging` to `prod`. The behavior is controlled entirely by environment variables.
- **Fail Fast**: The very first thing the application does on startup is validate its configuration. If a setting is invalid, it fails immediately with a clear error message. This has saved me countless hours of debugging mysterious runtime failures.

### The Engine Room: A Resilient Proxy Server

This is where all the components come together. In `internal/proxy/proxy.go`, I use the [Gin framework](https://gin-gonic.com/) to orchestrate the flow of requests. It's fast, reliable, and has great support for middleware.

I'm a big proponent of **Dependency Injection**, so I made sure the server doesn't create its own dependencies (like the cache or logger). Instead, we _inject_ them when the server is created. This is a crucial pattern that makes our components loosely coupled and a breeze to unit test, because you can just pass in mocks.

```go

func New(cfg *config.Config, cache cache.Cache, logger logger.Logger) (*Server, error) {
    // ... initialization ...

	engine.Use(
		middleware.RequestID(),       
		middleware.Logger(logger),    
		middleware.CORS(cfg.EnableCORS), 
		middleware.Security(),       
		middleware.Metrics(),        
	)

	return server, nil
}
```

- **The Middleware Pipeline**: I think of middleware as an assembly line for our requests. Every request that comes in passes through a standard set of steps: it gets a unique ID for tracing, it's logged, security headers are added, and more. This keeps my core proxy logic clean and focused on its main job: caching.
- **Zero-Downtime Deployments**: In production, you can't just pull the plug on a server. I built the proxy to listen for shutdown signals (`SIGINT`, `SIGTERM`) and perform a graceful shutdown. It stops accepting new requests but gives in-flight requests a chance to finish. For me, this is a non-negotiable feature for any serious service.

### The Watchful Eye: A Trilogy of Observability

I have a simple rule: if I can't see what a service is doing, I don't trust it in production. That's why I built the proxy to be transparent, giving us a clear view into its health and behavior through three key mechanisms.

1.  **Structured Logging**: I chose [Zerolog](https://github.com/rs/zerolog) to write logs as structured JSON. This is a game-changer. Every log line is a machine-readable event, enriched with context like the `request_id` and `duration`. When you're debugging an issue at 3 AM, being able to filter and search these logs in a platform like Splunk or Datadog is a lifesaver.

    ```go

    log.Info().
        Str("method", c.Request.Method).
        Str("path", c.Request.URL.Path).
        Int("status", c.Writer.Status()).
        Str("request_id", requestID).
        Dur("duration", duration).
        Msg("Request processed")
    ```

2.  **Health Probes**: I included Kubernetes-native health endpoints: `/health/live` (is the process running?) and `/health/ready` (can it serve traffic?). This is the language we use to let orchestrators like Kubernetes automatically manage the service, enabling a self-healing system.

3.  **Metrics**: I added a hook to export key metrics. This is where we'd track things like cache hit ratios, request latencies, and error rates. This is the data we feed into tools like Prometheus and Grafana to build dashboards and set up alerts, so we know about a problem before our users do.

### The Safety Net: Predictable Error Handling

I can't stand APIs that return vague or inconsistent errors. To solve this, I implemented a custom `AppError` type for all our error responses.

```go

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Err        error  `json:"-"`
}
```

This guarantees that every error is a clean, structured JSON object with a machine-readable `code`, a human-readable `message`, and the correct HTTP `StatusCode`. This makes life better for the developers consuming our API and makes my debugging process far more straightforward.

### The Conductor: Orchestrating the Application Lifecycle

I think of the `main.go` file as the conductor of our orchestra. Its job isn't to _do_ the work itself, but to make sure all the other components are initialized in the right order and that the service starts and stops cleanly.

```go

func main() {
	cfg, err := config.ParseFlags()

	log := logger.NewWithLevel(logLevel)
	cacheInstance := cache.New(cacheConfig)

	server, err := proxy.New(cfg, cacheInstance, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() { server.Start() }()

	<-ctx.Done()

	server.Shutdown(shutdownCtx)
}
```

Thanks to the modular design I chose, the `main` function is incredibly simple and readable. It tells a clear story about the service's lifecycle without getting bogged down in the implementation details of each component.

---

## From Code to Cloud: My Blueprint for Operations

I believe that great code is only half the story. A service is only as good as its operational story—how we build, deploy, and manage it. I designed this proxy from day one with a smooth operational lifecycle in mind.

### My Toolkit: Taskfile for Automation

I've moved on from Makefiles. For this project, I used `Taskfile.yml` for a modern, declarative approach to build automation. For me, this is all about consistency.

```yaml
# Taskfile.yml

version: "3"

tasks:
  build:
    desc: "Build the application binary."
    cmds:
      - go build -o caching-proxy ./cmd/caching-proxy

  test:
    desc: "Run all unit tests."
    cmds:
      - go test -v ./...

  lint:
    desc: "Run the golangci-lint linter."
    cmds:
      - golangci-lint run

  docker:
    desc: "Build the Docker container image."
    cmds:
      - docker build -t cache-proxy:latest .
```

The `Taskfile` gives us a single, self-documenting entry point for every common task. It guarantees that the command I run on my laptop is the _exact same command_ our CI server runs, which eliminates that whole class of "works on my machine" problems.

### The Vessel: A Production-Ready Dockerfile

I designed this service to be deployed as a container, and the `Dockerfile` I wrote uses a multi-stage build. In my opinion, this is a non-negotiable best practice for creating lean and secure production images.

```dockerfile

FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /caching-proxy ./cmd/caching-proxy

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /caching-proxy .

CMD ["./caching-proxy"]
```

- **Lean and Mean**: The final image I produce starts from a minimal `alpine` base and copies _only_ the compiled Go binary. No source code, no build tools. This drastically reduces the image size and its attack surface.
- **Completely Static**: I use `CGO_ENABLED=0` to build a statically-linked binary. This means it has zero dependencies on the host OS, making the container incredibly portable.

### Thriving in the Kubernetes Ecosystem

I engineered this service to be a first-class citizen in a Kubernetes environment. The features we've built map directly to the core concepts I rely on for cloud-native operations:

- Our 12-Factor design allows for **Declarative Configuration** via `ConfigMaps` and `Secrets`.
- The `/health/live` and `/health/ready` probes enable **Automated Healing**, which Kubernetes uses to manage rolling updates and restarts.
- My graceful shutdown logic supports **High Availability**, ensuring pods can be terminated without dropping user connections.
- The bounded, in-memory cache allows for **Stable Performance** by setting predictable resource limits.

## Fortifying the Gates: My Approach to Security

For me, security isn't a feature you bolt on at the end; it's a foundational requirement. I used a defense-in-depth strategy for this proxy.

### The First Line of Defense: Security Middleware

I wrote a simple but powerful gatekeeper in `middleware.Security()` that applies critical HTTP security headers to every response.

```go

func Security() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}
```

These headers instruct browsers to enable built-in protections against common attacks like Cross-Site Scripting (XSS) and clickjacking.

### The Principle of Least Privilege

I configured the `Dockerfile` to run the service as a non-root user, but in a real production environment, I'd take it even further in the Kubernetes manifest:

```yaml

securityContext:
  runAsNonRoot: true
  runAsUser: 1001
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
```

This is a powerful security control. If an attacker finds a vulnerability, their blast radius is tiny. They're trapped as a low-privilege user with a read-only filesystem.

## Architecting for Tomorrow: The Path to Hyperscale

A great design solves today's problems while keeping an eye on tomorrow's. The modular, interface-driven design I chose provides a clear roadmap for future evolution.

### Beyond a Single Node: Distributed Caching

The `InMemoryCache` is fast, but its state is local to a single pod. At scale, this leads to cache fragmentation. The beauty of the `cache.Cache` interface I defined is that we can swap in a `RedisCache` implementation without changing a single line of the proxy server.

```go

type RedisCache struct {
    client *redis.Client
}

func (r *RedisCache) Get(key string) (*Entry, bool) {
}

func (r *RedisCache) Set(key, value string, ttl time.Duration) {
}

server, err := proxy.New(cfg, cacheInstance, log)
```

By simply changing the component we inject, we can transform the service from a node-local cache to a globally consistent, distributed caching tier.

### Embracing the Service Mesh

As a microservices ecosystem grows, managing concerns like mTLS or retries in every service becomes a nightmare. A service mesh like Istio or Linkerd can offload this complexity to the infrastructure layer.

I designed our proxy to be a perfect fit for a service mesh. Its commitment to observability provides the rich data that meshes need to operate. We could even simplify our application by removing logic for things like mTLS, delegating them entirely to the mesh.

This forward-looking design—built for composition and integration—is a hallmark of modern enterprise architecture.

---

## The Crucible of Quality: My Testing Strategy

Code without tests is just a bug waiting to happen. My philosophy isn't about chasing 100% code coverage, but about achieving maximum confidence with a smart, layered testing strategy.

### The Foundation: Unit Tests

For me, unit tests are the bedrock. They're fast, isolated, and verify that each individual component does its job correctly. For example, I have tests that hammer the cache key generation logic to ensure it's deterministic.

```go

func TestGenerateKey(t *testing.T) {
    c := New(Config{DefaultTTL: 1 * time.Minute})

    key1 := c.GenerateKey("GET", "/users/123", "sort=asc")
    key2 := c.GenerateKey("GET", "/users/123", "sort=asc")
    key3 := c.GenerateKey("GET", "/users/123", "sort=desc")

    assert.Equal(t, key1, key2, "Keys with identical inputs should be identical")
    assert.NotEqual(t, key1, key3, "Keys with different inputs should not be identical")
}
```

This gives me high confidence that the building blocks of our system are solid.

### The Seams: Integration Tests

Next, I use integration tests to make sure our components play nicely together. They focus on the contracts between our packages, like verifying that the proxy server calls the cache module correctly.

```go

func TestProxyHandler_CacheInteraction(t *testing.T) {
    mockCache := new(mocks.Cache)
    logger := logger.New()
    cfg := &config.Config{Origin: "http://example.com"}
    server, _ := New(cfg, mockCache, logger)

    mockCache.On("Get", mock.Anything).Return(nil, false).Once()

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    server.engine.ServeHTTP(w, req)

    mockCache.AssertCalled(t, "Get", mock.Anything)
}
```

By using a real HTTP server but a _mocked_ `Cache` interface, I can validate this interaction without the brittleness of a full end-to-end test. It's a powerful way I test the seams of the architecture.

### The Full Picture: End-to-End (E2E) Tests

Finally, I use E2E tests for the ultimate reality check. They treat the entire application as a black box and verify a real user journey. For us, the most critical journey is the cache HIT/MISS cycle.

```bash

./caching-proxy --origin="http://localhost:9090" &
PROXY_PID=$!

RESPONSE_ONE=$(curl -s -I http://localhost:8080/data)
echo "$RESPONSE_ONE" | grep "X-Cache: MISS"

RESPONSE_TWO=$(curl -s -I http://localhost:8080/data)
echo "$RESPONSE_TWO" | grep "X-Cache: HIT"

kill $PROXY_PID
```

This layered strategy gives me the confidence to deploy quickly and safely.

## The Art of the Trade-off: My Engineering Decisions

Great engineering isn't about finding perfect solutions; it's about making smart, deliberate compromises. Here are a few key trade-offs I made during this project.

- **In-Memory Cache vs. Distributed Cache**: I chose a simple, in-memory cache. The benefit is blistering speed and zero operational overhead. The compromise is that the cache isn't shared between instances. My rationale? It's the simplest thing that works, and our interface-driven design means we can easily swap to Redis later if we need to.

- **Simple Eviction vs. True LRU**: I decided to evict the oldest item, not the least recently used. Why? Because a truly performant, thread-safe LRU is a complex beast. My simple policy is good enough for most workloads and keeps the code radically simpler and easier to maintain. I chose pragmatism over algorithmic perfection.

## A Final Thought: Your Turn to Build

We've journeyed from a high-level vision to the real-world trade-offs that define professional engineering. I've shown you how I built this proxy, but this is more than just a service; it's a collection of battle-tested patterns for building software that lasts.

But this isn't the end of the story. I see it as a challenge. How would you improve it? Could you build a smarter eviction policy? Integrate it with Prometheus and Grafana? The path to mastery is paved with relentless curiosity. Take these ideas, challenge them, and go build something great.

---

## References

- The initial project idea was inspired by the "Caching Server in Go" project on [roadmap.sh](https://roadmap.sh/projects/caching-server).
- The complete source code for this project is available on [GitHub](https://github.com/rowjay007/cache-proxy).
