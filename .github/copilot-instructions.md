# OpenAI Compatible Proxy - AI Agent Instructions

## Architecture Overview

This is a **Go-based proxy service** that provides unified API access to LM Studio (local LLM server), translating between three API formats: OpenAI, Ollama, and LM Studio native. The proxy adds intelligent features like Wake-on-LAN for sleeping hosts and idle monitoring for energy efficiency.

### Key Components

- **Controller Layer** (`internal/controller/`): Gin HTTP handlers for three API surfaces:
  - `/v1/*` - OpenAI-compatible API (proxy + `/v1/models` override)
  - `/api/tags`, `/api/show`, `/api/version` - Ollama API
  - `/api/v0/models` - LM Studio native API
- **Service Layer** (`internal/core/service/`):
  - `lm_studio_service.go`: Core proxy logic with WOL support and model syncing
  - `idle_monitor_service.go`: Singleton that sends MQTT suspend commands after inactivity
  - `model_service.go`: Model CRUD operations with MongoDB
- **Port/Interface Layer** (`internal/core/port/`): Service interfaces (e.g., `ILMStudioService`) for dependency injection

- **Infrastructure Layer** (`internal/infrastructure/`):
  - `api/lm_studio_api.go`: HTTP client for LM Studio backend
  - `repository/model_repository.go`: MongoDB persistence
  - `config/config.go`: YAML config loader from `conf/application.yml`

### Data Flow

1. Request arrives at controller → records activity in idle monitor singleton
2. Middleware checks if path should be intercepted (e.g., `/v1/models`) or proxied
3. For proxied requests: `LMStudioService.ProxyRequestStreaming()` forwards to LM Studio with streaming support
4. On connection failure: Automatic WOL packet sent, retries with exponential backoff
5. Model list endpoints fetch from MongoDB (synced periodically from LM Studio)

## Critical Patterns

### Routing with Selective Proxying

Routes use **middleware-based interception** before catch-all proxy handlers:

```go
// Example from router/router.go
routerV1.Use(func(c *gin.Context) {
    if c.Request.URL.Path == "/v1/models" && c.Request.Method == "GET" {
        openAIController.ListModels(c)
        c.Abort()  // Prevents proxy handler from running
        return
    }
    c.Next()
})
routerV1.Any("/*any", lmStudioProxyController.ProxyRequest)
```

**When adding new endpoints**: Check if path needs custom handling (intercept) or should be proxied transparently.

### Wake-on-LAN Auto-Recovery

Connection failures trigger automatic host wakeup:

```go
// In lm_studio_service.go - doRequestWithWOL()
// 1. First request attempt
// 2. If connection error → send WOL magic packet
// 3. Wait retryWait duration (default 5s)
// 4. Retry maxRetries times (default 3)
// 5. Return error if still failing
```

Config: `conf/application.yml` under `lm-studio.wol.*`. Requires valid MAC address.

### Idle Monitoring Singleton

Prevents multiple monitor instances:

```go
// Always use GetIdleMonitor() - never create new instances
GetIdleMonitor().RecordActivity()  // Call on ANY user interaction
```

Monitors last activity timestamp. After `mqtt.idle.timeout`, publishes suspend message once (flag prevents spam).

### Context Propagation

All services use `context.Context` as first parameter. Extract from Gin context:

```go
ctx := GetContext(ginCtx)  // Defined in controller/middleware.go
```

## Development Workflows

### Build & Run Locally

```bash
# Install dependencies
go mod download

# Build binary
go build -o bin/app .

# Run (requires MongoDB + MQTT or set mqtt.enabled=false)
./bin/app
```

### Run with Docker Compose

```bash
docker-compose up -d  # Starts app + MongoDB + Mosquitto MQTT
docker-compose logs -f backend  # View app logs
```

### Regenerate Swagger Docs

```bash
# After changing @-annotations in controllers
swag init --parseDependency --parseInternal

# Docs available at http://localhost:8080/swagger-ui/
```

Access via `http://localhost:8080/swagger-ui/index.html`

### Testing

```bash
go test -v ./...  # Run all tests
go vet ./...      # Static analysis
```

## Configuration

All settings in `conf/application.yml`:

- **server.listening**: Bind address (default `0.0.0.0:8080`)
- **lm-studio.url**: Backend LM Studio server URL
- **lm-studio.wol**: Wake-on-LAN settings (enable, MAC address, retries)
- **mqtt.idle.timeout**: Time before sending suspend (e.g., `10m`)
- **data.mongo.uri**: MongoDB connection string

Environment-specific profiles via `PROFILE` env var (defaults to `dev`).

## Common Mistakes to Avoid

1. **Don't create controllers/services manually** - Use dependency injection from `router/router.go`
2. **Don't buffer streaming responses** - Use `io.Copy()` for LLM token streaming
3. **Don't forget idle tracking** - Call `GetIdleMonitor().RecordActivity()` on user actions
4. **Don't hardcode timeouts** - Use `lm-studio.timeout` from config (default 600s for long LLM generations)
5. **Model sync is pull-based** - Models fetched from LM Studio, not pushed

## Key Files Reference

- `main.go`: Entry point - loads config, connects DBs, starts idle monitor, launches HTTP server
- `internal/core/common/router/router.go`: All route registration and middleware setup
- `internal/controller/*_proxy_controller.go`: Streaming proxy implementations
- `internal/infrastructure/config/config.go`: Config struct definitions matching YAML
- `docs/swagger.{json,yaml}`: Auto-generated API docs (don't edit manually)

## External Dependencies

- **LM Studio**: Local LLM server (configurable URL, typically port 11434)
- **MongoDB**: Model metadata persistence
- **Mosquitto MQTT**: Optional idle suspend messaging (can be disabled)
- **Go 1.25**: Module-based dependency management (`go.mod`)

## Multi-Platform Support

Dockerfile uses `TARGETPLATFORM` build args for multi-arch (amd64/arm64). GitHub Actions CI builds both platforms separately for speed.
