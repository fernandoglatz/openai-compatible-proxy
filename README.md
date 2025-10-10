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

The application uses a configuration file `conf/application.yml` for settings:

```yaml
server:
  listening: "0.0.0.0:8080"
  context-path: "/openai-compatible-proxy"

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

log:
  level: TRACE
  format: TEXT
  colored: true
```

### Running the Application

#### Option 1: Direct Execution (Local)

```bash
go run main.go
```

#### Option 2: Docker Container (Recommended)

1. Build and start with docker-compose:
   ```bash
   docker-compose up -d
   ```

2. The service will be available at `http://localhost:8080/openai-compatible-proxy`

## API Endpoints

### Health Check
- **GET** `/health` - Health check endpoint

### Models
- **GET** `/model` - Get all models
- **GET** `/model/{id}` - Get a specific model by ID
- **GET** `/model/lm-studio` - Fetch and save models from LM Studio

### OpenAI API Compatibility
The proxy provides standard OpenAI-compatible endpoints:
- **POST** `/v1/chat/completions`
- **POST** `/v1/models`
- And other standard OpenAI API endpoints

### Ollama API Compatibility  
- **GET** `/api/tags` - Get available models
- **GET** `/api/version` - Get version information
- **POST** `/api/show` - Get details of a specific model

## Usage Examples

### Get All Models
```bash
curl http://localhost:8080/openai-compatible-proxy/model
```

### Fetch LM Studio Models
```bash
curl http://localhost:8080/openai-compatible-proxy/model/lm-studio
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