# OpenAI Compatible Proxy

[![CI](https://github.com/fernandoglatz/openai-compatible-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/fernandoglatz/openai-compatible-proxy/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/fernandoglatz/openai-compatible-proxy)](go.mod)

A smart proxy service that provides unified API access to [LM Studio](https://lmstudio.ai/) through multiple API formats: OpenAI, Ollama, and LM Studio native. Built with energy efficiency in mind, featuring automatic Wake-on-LAN for sleeping hosts and intelligent idle monitoring.

## ✨ Features

- 🔄 **Multi-API Support**: Translate between OpenAI, Ollama, and LM Studio API formats
- 🌊 **Streaming Support**: Efficient token streaming for real-time LLM responses
- 🔌 **Wake-on-LAN**: Automatically wake sleeping LM Studio hosts on-demand
- ⚡ **Idle Monitoring**: Send MQTT suspend messages after inactivity to save energy (pairs with [mqtt-system-agent](https://github.com/fernandoglatz/mqtt-system-agent))
- 💾 **Model Caching**: MongoDB-based model metadata persistence
- 📚 **Swagger Documentation**: Interactive API documentation at `/swagger-ui/`
- 🐳 **Docker Support**: Multi-platform images (amd64/arm64) with Docker Compose
- 🔒 **Configurable Security**: Support for basic auth and API key authentication

## 🚀 Quick Start

### Using Docker Compose (Recommended)

1. Clone the repository:
```bash
git clone https://github.com/fernandoglatz/openai-compatible-proxy.git
cd openai-compatible-proxy
```

2. Configure `conf/application.yml`:
```yaml
lm-studio:
  url: "http://your-lm-studio-host:11434"  # Update with your LM Studio URL
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

### Building from Source

**Prerequisites:**
- Go 1.25 or later
- MongoDB (optional, can be disabled)
- Mosquitto MQTT (optional, can be disabled)

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
```bash
# List models
curl http://localhost:8080/v1/models

# Chat completion (proxied to LM Studio)
curl http://localhost:8080/v1/chat/completions \
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

lm-studio:
  url: "http://localhost:1234"
  timeout: 600s  # Long timeout for LLM generation
  wol:
    enabled: true
    mac-address: "00:00:00:00:00:00"
    broadcast-address: "255.255.255.255:9"
    max-retries: 10
    retry-wait: 5s

mqtt:
  enabled: true
  broker: "tcp://mqtt:1883"
  client-id: "openai-compatible-proxy"
  username: "guest"
  password: "guest"
  topic: "system/suspend"  # Must match topic in mqtt-system-agent
  qos: 1
  idle:
    timeout: 10m  # Send suspend after 10 minutes of inactivity
    message: "suspend"  # Message received by mqtt-system-agent

log:
  level: TRACE  # DEBUG, INFO, WARN, ERROR
  format: TEXT  # TEXT or JSON
  colored: true
```

### Environment Variables

- `PROFILE`: Set environment profile (defaults to `dev`)
- `TZ`: Timezone (e.g., `America/Sao_Paulo`)

## 🏗️ Architecture

The project follows a clean hexagonal architecture:

```
internal/
├── controller/          # HTTP handlers (Gin)
├── core/
│   ├── entity/         # Domain entities
│   ├── model/          # DTOs and requests/responses
│   ├── port/           # Service interfaces
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
- **Idle Monitor**: Singleton service tracking activity and sending suspend messages via MQTT
- **Model Service**: CRUD operations for model metadata in MongoDB

### Energy-Efficient Workflow

1. **On Request**: If LM Studio host is asleep, proxy sends Wake-on-LAN magic packet
2. **During Use**: Proxy forwards requests to LM Studio and resets idle timer
3. **After Idle**: When `idle.timeout` expires, proxy publishes suspend message to MQTT
4. **System Sleep**: [mqtt-system-agent](https://github.com/fernandoglatz/mqtt-system-agent) on LM Studio host receives message and suspends the system

This creates a fully automated power management cycle for your local LLM infrastructure.

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

- The proxy supports basic authentication and API key authentication
- Configure authentication in your reverse proxy or API gateway
- Use MongoDB authentication in production
- Secure MQTT broker with proper credentials
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

## � Related Projects

- **[MQTT System Agent](https://github.com/fernandoglatz/mqtt-system-agent)** - Companion service that runs on your LM Studio host to receive MQTT suspend messages and automatically suspend the system after idle timeout. Cross-platform support for Linux and Windows with easy service installation.

## �🙏 Acknowledgments

- [LM Studio](https://lmstudio.ai/) - Local LLM hosting
- [Gin](https://github.com/gin-gonic/gin) - HTTP web framework
- [MongoDB](https://www.mongodb.com/) - Database
- [Eclipse Mosquitto](https://mosquitto.org/) - MQTT broker

## 📬 Support

- **Documentation**: [API.md](API.md) for detailed API reference
- **Issues**: [GitHub Issues](https://github.com/fernandoglatz/openai-compatible-proxy/issues)
- **Discussions**: [GitHub Discussions](https://github.com/fernandoglatz/openai-compatible-proxy/discussions)

---

**Note**: This proxy is designed for local/private LLM deployments. For production use with sensitive data, ensure proper security measures are in place.
