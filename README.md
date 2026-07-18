# OpenAI Compatible Proxy

[![CI](https://github.com/fernandoglatz/openai-compatible-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/fernandoglatz/openai-compatible-proxy/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/fernandoglatz/openai-compatible-proxy)](go.mod)

A smart proxy service that provides unified API access to [LM Studio](https://lmstudio.ai/) through multiple API formats: OpenAI, Ollama, and LM Studio native. Built with energy efficiency in mind, featuring automatic Wake-on-LAN for sleeping hosts.

## ✨ Features

- 🔄 **Multi-API Support**: Translate between OpenAI, Ollama, and LM Studio API formats
- 🌊 **Streaming Support**: Efficient token streaming for real-time LLM responses
- 🔌 **Wake-on-LAN**: Automatically wake sleeping LM Studio hosts on-demand
- 💾 **Model Caching**: MongoDB-based model metadata persistence
- 📚 **Swagger Documentation**: Interactive API documentation at `/swagger-ui/`
- 🐳 **Docker Support**: Multi-platform images (amd64/arm64) with Docker Compose
- 🔒 **Token Authentication**: Multiple revocable Bearer tokens guarding the OpenAI API
- 🚦 **Sticky-Session Scheduler**: Serialize generation requests so the local model keeps one agent's context warm, switching sessions only after an idle window or a completed turn

## 🚀 Quick Start

### Using Docker Compose (Recommended)

1. Clone the repository:
```bash
git clone https://github.com/fernandoglatz/openai-compatible-proxy.git
cd openai-compatible-proxy
```

2. Configure `conf/application.yml`:
```yaml
openai:
  api-keys:                 # Tokens accepted by the proxy on /v1/* and /api/v1/*
    - "sk-my-first-token"
    - "sk-my-second-token"

lm-studio:
  url: "http://your-lm-studio-host:1234"   # Update with your LM Studio URL
  api-key: ""                              # Only if your LM Studio requires one
  wol:
    enabled: true
    mac-address: "XX:XX:XX:XX:XX:XX"  # Your LM Studio host MAC address
```

3. Start the stack:
```bash
docker-compose up -d
```

4. Access the service:
- API: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger-ui/`

### Sticky-Session Scheduler

When multiple agents (e.g. opencode subagents) share one local model, interleaving
their requests forces expensive context re-evaluation on every switch. The scheduler
runs one generation at a time and keeps the active session warm:

```yaml
scheduler:
  enabled: true
  idle-timeout: 10s          # how long the active session keeps priority when idle
  gated-paths:
    - /v1/chat/completions
    - /v1/completions
    - /v1/responses
```

Sessions are identified by the `X-Session-Id` request header (sent by opencode). A
waiting session takes over only after the active session finishes a turn (`stop`) or
stays idle past `idle-timeout`; a turn ending in `tool_calls` holds the slot so the
agent can continue after running its tool.

### Building from Source

**Prerequisites:**
- Go 1.25 or later
- MongoDB (required — the app exits at startup if it cannot connect)

```bash
# Install dependencies
go mod download

# Build the binary
go build -o bin/app .

# Run
./bin/app
```

## 📖 API Endpoints

### OpenAI Compatible API

These routes require a Bearer token from `openai.api-keys` (see [Authentication](#-authentication)).

```bash
# List models
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer sk-my-first-token"

# Chat completion (proxied to LM Studio)
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-my-first-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Ollama API
```bash
# List models
curl http://localhost:8080/api/tags

# Show model details
curl http://localhost:8080/api/show \
  -H "Content-Type: application/json" \
  -d '{"model": "your-model"}'

# Get version
curl http://localhost:8080/api/version
```

### LM Studio Native API
```bash
# List models
curl http://localhost:8080/api/v0/models

# Get specific model
curl http://localhost:8080/api/v0/models/{model-id}
```

### LM Studio native v1 API (LM Studio 0.4.0+)

Authenticated — send a token from `openai.api-keys` when any are configured.

```bash
# List models (served from the local store, so it works while the host is asleep)
curl http://localhost:8080/api/v1/models \
  -H "Authorization: Bearer $TOKEN"

# Load a model
curl http://localhost:8080/api/v1/models/load \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model": "google/gemma-4-26b-a4b"}'

# Chat (stateful; pass previous_response_id to continue a conversation)
curl http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model": "google/gemma-4-26b-a4b", "messages": [{"role": "user", "content": "Hi"}]}'
```

## ⚙️ Configuration

All configuration is managed through `conf/application.yml`:

```yaml
server:
  listening: "0.0.0.0:8080"
  context-path: "/"

data:
  mongo:
    uri: "mongodb://mongo:27017"
    database: "openai-compatible-proxy"

openai:
  api-keys: []   # Tokens accepted by the proxy; empty disables authentication

lm-studio:
  url: "http://localhost:1234"
  api-key: ""    # Single token sent upstream to LM Studio
  timeout: 600s  # Long timeout for LLM generation
  wol:
    enabled: true
    mac-address: "00:00:00:00:00:00"
    broadcast-address: "255.255.255.255:9"
    max-retries: 10
    retry-wait: 5s

log:
  level: TRACE  # DEBUG, INFO, WARN, ERROR
  format: TEXT  # TEXT or JSON
  colored: true
```

## 🔐 Authentication

The proxy accepts **multiple** tokens, so each client can have its own token that you
can rotate or revoke independently. LM Studio upstream uses a **single** key.

```yaml
openai:
  api-keys:
    - "sk-laptop-token"
    - "sk-phone-token"

lm-studio:
  api-key: ""   # optional, only if your LM Studio requires a key
```

Clients authenticate with a standard OpenAI Bearer token, so any OpenAI SDK works
unchanged by pointing `base_url` at the proxy and `api_key` at one of your tokens:

```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer sk-laptop-token"
```

Behavior:

- **Scope**: the `/v1/*` and `/api/v1/*` routes are authenticated. The Ollama routes
  (`/api/tags`, `/api/show`, `/api/version`), the LM Studio legacy routes (`/api/v0/*`),
  `/health`, and `/swagger-ui/` remain open.
- **Empty `api-keys`**: authentication is disabled and a warning is logged at startup.
- **Rejection**: a missing or unknown token gets `401` with an OpenAI-shaped error
  body (`invalid_api_key`), which OpenAI SDK clients parse natively.
- **Token isolation**: the caller's `Authorization` header is stripped before the
  request is forwarded, and replaced with `lm-studio.api-key` when configured. Your
  proxy tokens are never exposed to LM Studio.

### Environment Variables

- `PROFILE`: Set environment profile (defaults to `dev`)
- `TZ`: Timezone (e.g., `America/Sao_Paulo`)

## 🏗️ Architecture

The project follows a clean hexagonal architecture:

```
internal/
├── controller/          # HTTP handlers (Gin)
├── core/
│   ├── common/         # Route setup, logging, shared utils
│   ├── entity/         # Domain entities
│   ├── model/          # DTOs and requests/responses
│   ├── port/           # Service and repository interfaces
│   ├── service/        # Business logic
│   └── server/         # HTTP server setup
└── infrastructure/
    ├── api/            # External API clients
    ├── config/         # Configuration management
    └── repository/     # Data persistence
```

### Key Components

- **Proxy Controller**: Forwards requests to LM Studio with streaming support
- **LM Studio Service**: Handles WOL, retries, and model synchronization
- **Model Service**: CRUD operations for model metadata in MongoDB

### Energy-Efficient Workflow

1. **On Request**: If LM Studio host is asleep, proxy sends Wake-on-LAN magic packet
2. **During Use**: Proxy forwards requests to LM Studio

This lets the LM Studio host sleep when unused and wake on demand.

## 🔧 Development

### Run Tests
```bash
go test -v ./...
```

### Static Analysis
```bash
go vet ./...
```

### Regenerate Swagger Docs
```bash
swag init --parseDependency --parseInternal
```

### View Logs
```bash
# Docker Compose
docker-compose logs -f backend

# Direct binary
./bin/app  # Logs to stdout
```

## 🐳 Docker

### Pull Pre-built Image
```bash
docker pull ghcr.io/fernandoglatz/openai-compatible-proxy:latest
```

### Build Multi-Platform Image
```bash
docker buildx build --platform linux/amd64,linux/arm64 -t openai-compatible-proxy:latest .
```

## 🔐 Security Considerations

- Set `openai.api-keys` before exposing the proxy beyond localhost; when it is empty
  the `/v1/*` and `/api/v1/*` routes are unauthenticated (see [Authentication](#-authentication))
- Only `/v1/*` and `/api/v1/*` are authenticated. Put the Ollama (`/api/tags`, `/api/show`,
  `/api/version`) and LM Studio legacy (`/api/v0/*`) routes behind a reverse proxy or
  network isolation if they should not be public
- Terminate TLS at a reverse proxy — over plain HTTP, Bearer tokens travel in cleartext
- Treat `conf/application.yml` as a secret; do not commit real tokens
- Use MongoDB authentication in production
- Consider network isolation for LM Studio host

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [LM Studio](https://lmstudio.ai/) - Local LLM hosting
- [Gin](https://github.com/gin-gonic/gin) - HTTP web framework
- [MongoDB](https://www.mongodb.com/) - Database

## 📬 Support

- **Documentation**: [API.md](API.md) for detailed API reference
- **Issues**: [GitHub Issues](https://github.com/fernandoglatz/openai-compatible-proxy/issues)
- **Discussions**: [GitHub Discussions](https://github.com/fernandoglatz/openai-compatible-proxy/discussions)

---

**Note**: This proxy is designed for local/private LLM deployments. For production use with sensitive data, ensure proper security measures are in place.
