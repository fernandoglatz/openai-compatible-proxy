# OpenAI Compatible Proxy

This is a Go-based proxy server that provides an OpenAI-compatible API interface. It supports integration with LM Studio and Ollama models, allowing you to interact with various LLMs through a standardized API endpoint.

## Features

- **OpenAI API Compatibility**: Provides endpoints compatible with the OpenAI API specification
- **Model Management**: Supports multiple model types (LLM/VLM) with detailed metadata
- **LM Studio Integration**: Fetches and synchronizes models from LM Studio
- **Ollama Support**: Provides Ollama-compatible API endpoints
- **MongoDB Storage**: Persistent storage for model information
- **Redis Caching**: Caching for improved performance
- **Docker Support**: Easy deployment with Docker and docker-compose

## Architecture

The application follows a layered architecture:
- **Controller Layer**: Handles HTTP requests and responses
- **Service Layer**: Contains business logic and model operations
- **Repository Layer**: Data access layer with MongoDB
- **Infrastructure Layer**: Configuration, database connections, logging
- **Entity Layer**: Model definitions

## Getting Started

### Prerequisites

- Go 1.25 or higher
- Docker and Docker Compose (for containerized deployment)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/fernandoglatz/openai-compatible-proxy.git
   cd openai-compatible-proxy
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o main .
   ```

### Configuration

The application uses a configuration file `conf/application.yml` for settings. **You must create this file and customize it for your environment before running the application.**

Example `conf/application.yml`:

```yaml
server:
  listening: "0.0.0.0:8080"
  context-path: "/"

data:
  mongo:
    uri: "mongodb://mongo:27017"
    database: "openai-compatible-proxy"

  redis:
    address: "redis:6379"
    password: ""
    db: 0
    ttl:
      model: 24h

lm-studio:
  url: "http://localhost:1234"
  timeout: 600s
  wol:
    enabled: true
    mac-address: "00:00:00:00:00:00"  # MAC address for Wake-on-LAN
    broadcast-address: "255.255.255.255:9"
    max-retries: 10
    retry-wait: 5s

mqtt:
  enabled: true
  broker: "tcp://mqtt:1883"
  client-id: "openai-compatible-proxy"
  username: "guest"
  password: "guest"
  topic: "system/suspend"
  qos: 1
  idle:
    timeout: 10m
    message: "suspend"

log:
  level: TRACE  # Options: TRACE, DEBUG, INFO, WARN, ERROR
  format: TEXT  # Options: TEXT, JSON
  colored: true
```

#### Configuration Options:

- **server.listening**: The address and port the server listens on
- **server.context-path**: Base path for all API endpoints
- **data.mongo**: MongoDB connection settings for persistent storage
- **data.redis**: Redis connection settings for caching
- **lm-studio**: LM Studio integration settings including Wake-on-LAN support
- **mqtt**: MQTT broker settings for idle monitoring and system suspend
- **log**: Logging configuration (level, format, colors)

### Running the Application

#### Option 1: Using Pre-built Docker Image (Recommended)

Pull and run the container directly from GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/fernandoglatz/openai-compatible-proxy:latest

# Run the container with your custom configuration
docker run -d \
  --name openai-compatible-proxy \
  -p 8080:8080 \
  -v ./conf:/app/conf \
  ghcr.io/fernandoglatz/openai-compatible-proxy:latest
```

**Note**: Make sure to create and customize your `conf/application.yml` file before running the container. See the [Configuration](#configuration) section below for details.

#### Option 2: Docker Compose (Full Stack)

1. Start the full stack (backend, MongoDB, Redis, and MQTT):
   ```bash
   docker-compose up -d
   ```

   This will pull the latest image from GitHub Container Registry and start all services.

2. The service will be available at `http://localhost:8080`

**Note**: The docker-compose configuration uses the pre-built image from GitHub Container Registry, so you don't need to build it locally.

#### Option 3: Direct Execution (Local Development)

```bash
go run main.go
```

## API Endpoints

The proxy provides multiple API compatibility layers to work with different LLM interfaces.

### Health Check
- **GET** `/health` - Health check endpoint
  ```bash
  curl http://localhost:8080/health
  ```

### OpenAI API Compatibility

All OpenAI-compatible endpoints are available under the `/v1` prefix. These endpoints are proxied to LM Studio:

- **GET** `/v1/models` - List available models (handled locally, not proxied)
  ```bash
  curl http://localhost:8080/v1/models
  ```

- **POST** `/v1/chat/completions` - Create a chat completion (proxied to LM Studio)
  ```bash
  curl -X POST http://localhost:8080/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{
      "model": "your-model-name",
      "messages": [{"role": "user", "content": "Hello!"}]
    }'
  ```

- **POST** `/v1/completions` - Create a text completion (proxied to LM Studio)
- **POST** `/v1/embeddings` - Create embeddings (proxied to LM Studio)
- All other OpenAI API endpoints are proxied to LM Studio

### LM Studio API Compatibility

LM Studio-specific endpoints under `/api/v0`:

- **GET** `/api/v0/models` - List available models in LM Studio format
  ```bash
  curl http://localhost:8080/api/v0/models
  ```

- **GET** `/api/v0/models/{model}` - Get details of a specific model by ID
  ```bash
  curl http://localhost:8080/api/v0/models/model-id
  ```

- All other `/api/v0/*` endpoints are proxied to LM Studio

### Ollama API Compatibility

Ollama-compatible endpoints under `/api`:

- **GET** `/api/tags` - Get available models in Ollama format
  ```bash
  curl http://localhost:8080/api/tags
  ```

- **GET** `/api/version` - Get version information
  ```bash
  curl http://localhost:8080/api/version
  ```

- **POST** `/api/show` - Get details of a specific model by name
  ```bash
  curl -X POST http://localhost:8080/api/show \
    -H "Content-Type: application/json" \
    -d '{"name": "model-name"}'
  ```

### Swagger Documentation

Interactive API documentation is available via Swagger UI:
- **GET** `/swagger-ui/index.html` - Access Swagger UI
  ```bash
  # Open in browser
  http://localhost:8080/swagger-ui/index.html
  ```

## Development

### Project Structure
- `main.go` - Entry point of the application
- `internal/` - Main source code:
  - `controller/` - HTTP request handlers
  - `service/` - Business logic
  - `repository/` - Data access layer
  - `model/` - Data models and DTOs
  - `config/` - Configuration management
- `conf/` - Configuration files
- `scripts/` - Utility scripts

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For support, please open an issue on the GitHub repository.
```