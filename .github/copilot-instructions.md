# OpenAI Compatible Proxy - Development Guide

This proxy server provides a unified interface for interacting with various LLM providers (LM Studio, Ollama, etc.) while offering additional features like model management and caching.

## Architecture Overview

This is a Go-based microservice that implements a layered architecture following clean architecture principles:

- **Presentation Layer**: HTTP controllers (`internal/controller`) handling API endpoints and request routing
- **Application Layer**: Services (`internal/core/service`) implementing business logic
- **Infrastructure Layer**: Repositories (`internal/infrastructure/repository`) for data persistence and external API integration
- **Core Layer**: Shared components, entities, and interfaces

## Key Components

### 1. Main Services

- **LM Studio Proxy**: Routes requests to LM Studio API with proper proxying
- **Ollama API**: Implements Ollama-compatible endpoints for model management
- **Model Management**: Centralized service for storing and retrieving LLM models

### 2. Data Storage

- **MongoDB**: Primary storage for LLM models with metadata

### 3. Configuration

Configuration is managed through:

- `conf/application.yml` - Main configuration file
- Environment variables for deployment-specific settings

## API Endpoints

### Core Routes

- `/model` - Get all models or specific model by ID
- `/model/lm-studio` - Fetch and save models from LM Studio
- `/health` - Health check endpoint

### Proxy Routes

- `/v1/*any` - LM Studio proxy endpoint
- `/api/tags` - Ollama tags endpoint
- `/api/show` - Ollama model details endpoint
- `/api/version` - Ollama version endpoint

## How to Run

1. Configure `conf/application.yml` with appropriate connections:
   - MongoDB connection string
   - LM Studio API URL and timeout
2. Set environment variables (if needed)
3. Build the application: `go build -o main .`
4. Run: `./main`

## Development Workflow

1. **Adding new LLM provider**: Create a new service that implements the appropriate interface
2. **Model persistence**: Use the model repository and service to manage models in MongoDB
3. **API endpoints**: Add new routes in controllers and update routing logic
4. **Configuration**: Modify `conf/application.yml` for new settings

## Integration Points

- **MongoDB**: Used for storing model definitions with metadata
- **LM Studio**: Primary LLM provider that this proxy forwards requests to
- **Ollama API**: Provides compatibility layer for Ollama-compatible clients

## Code Structure

```
internal/
├── controller/        # HTTP handlers and request routing
├── service/            # Business logic implementations
├── infrastructure/   # External API integrations and data access
└── core/            # Shared components, entities, and interfaces

conf/                 # Configuration files
scripts/              # Deployment and maintenance scripts
```

## Key Files

- `main.go` - Application entry point
- `internal/controller/lm_studio_proxy_controller.go` - LM Studio proxy endpoint
- `internal/controller/ollama_controller.go` - Ollama-compatible API endpoints
- `internal/service/lm_studio_service.go` - Logic for interacting with LM Studio
- `internal/infrastructure/repository/model_repository.go` - MongoDB operations for models
