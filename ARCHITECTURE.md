# CLAUDE.md
# ARCHITECT.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CLIProxyAPI is a Go-based proxy server that provides OpenAI/Gemini/Claude/Codex compatible API interfaces for CLI models. It enables local or multi-account CLI access with OpenAI/Gemini/Claude-compatible clients and SDKs through OAuth authentication.

**Key Features:**
- Multi-provider support: OpenAI, Gemini, Claude, Qwen, iFlow via OAuth
- Load balancing across multiple accounts
- Streaming and non-streaming responses
- Function calling/tools support
- Multimodal input support (text and images)
- Amp CLI and IDE extensions support
- Reusable Go SDK

## Architecture

```
├── cmd/server/main.go          # Main application entry point
├── internal/                   # Private application packages
│   ├── access/                 # Access control and authentication
│   ├── buildinfo/             # Build-time information
│   ├── cmd/                   # Command-line interface
│   ├── config/                # Configuration management
│   ├── logging/               # Logging infrastructure
│   ├── managementasset/       # Management API assets
│   ├── store/                 # Data storage layer
│   │   ├── gitstore.go        # Git-backed storage (GitHub integration)
│   │   ├── postgresstore.go   # PostgreSQL storage backend
│   │   └── objectstore.go     # S3-compatible object storage
│   ├── translator/            # API format translators
│   ├── usage/                 # Usage tracking
│   └── util/                  # Utility functions
├── sdk/                       # Public SDK for embedding
│   ├── auth/                  # SDK authentication
│   └── cliproxy/              # Core proxy functionality
└── docs/                      # Documentation
```

**Core Components:**
- **Main Server** (`cmd/server/main.go`): Entry point with version injection
- **Translators**: Convert between different AI API formats (OpenAI ↔ CLI models)
- **OAuth Integration**: Multi-provider authentication flows
- **Load Balancer**: Distributes requests across multiple accounts
- **Management API**: Localhost-only endpoints for account management
- **Storage Layer**: Three backend options for persisting configuration and authentication data

**Storage Layer Options:**
The application supports three different storage backends for persisting configuration and authentication data:

1. **Git-Backed Storage (GitHub/Git)**:
   - Stores data in a Git repository for version control and collaboration
   - Environment variables: `GITSTORE_GIT_URL`, `GITSTORE_GIT_USERNAME`, `GITSTORE_GIT_TOKEN`, `GITSTORE_LOCAL_PATH`
   - Implemented in `internal/store/gitstore.go`
   - Automatically commits and pushes changes to the remote repository

2. **PostgreSQL Storage**:
   - Stores data in PostgreSQL database tables for enterprise-grade persistence
   - Environment variables: `PGSTORE_DSN`, `PGSTORE_SCHEMA`, `PGSTORE_LOCAL_PATH`
   - Implemented in `internal/store/postgresstore.go`
   - Maintains local file mirroring for compatibility with existing workflows

3. **S3-Compatible Object Storage**:
   - Stores data in S3-compatible storage (AWS S3, MinIO, etc.) for cloud-native deployments
   - Environment variables: `OBJECTSTORE_ENDPOINT`, `OBJECTSTORE_BUCKET`, `OBJECTSTORE_ACCESS_KEY`, `OBJECTSTORE_SECRET_KEY`, `OBJECTSTORE_LOCAL_PATH`
   - Implemented in `internal/store/objectstore.go`
   - Supports any S3-compatible storage provider

## Common Development Commands

### Building and Running

**Local Development:**
```bash
# Build the main server
go build -o bin/cli-proxy-api cmd/server/main.go

# Run the server directly
go run cmd/server/main.go

# Load environment variables from .env file
go run cmd/server/main.go --config config.yaml
```

**Docker Development:**
```bash
# Build and run using provided script
./docker-build.sh

# Manual Docker commands
docker-compose build
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/config

# Run tests with verbose output
go test -v ./...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter (requires golangci-lint)
golangci-lint run

# Tidy dependencies
go mod tidy

# Vendor dependencies (if needed)
go mod vendor
```

### Development Setup

```bash
# Install dependencies
go mod download

# Copy and edit configuration
cp config.example.yaml config.yaml

# Set up environment variables
cp .env.example .env
```

## Configuration

The server uses YAML configuration files and environment variables:

- `config.yaml`: Main configuration file
- `.env`: Environment variables (API keys, database URLs)
- `config.example.yaml`: Template configuration

## Testing Strategy

- Unit tests are co-located with source files (`*_test.go`)
- Integration tests may be in separate test packages
- Use `go test ./...` to run the full test suite
- Mock external services for reliable testing

## Key Dependencies

- **Gin**: HTTP web framework for API endpoints
- **OAuth2**: Authentication for multiple AI providers
- **PostgreSQL**: Primary data storage via pgx
- **WebSocket**: Real-time streaming support
- **Logrus**: Structured logging
- **Gin**: HTTP routing and middleware
- **go-git**: Git operations for Git-backed storage
- **MinIO Go SDK**: S3-compatible object storage operations

## Storage Backend Configuration

The application determines which storage backend to use based on environment variables:

1. **Priority Order**: PostgreSQL → Object Store → Git Store → Local Files (default)
2. **Configuration**: Specified through environment variables in `.env` file
3. **Runtime Selection**: `cmd/server/main.go` checks for environment variables and initializes the appropriate storage backend
4. **Interface Consistency**: All storage backends implement the same interface for seamless switching


## Build Information

Build-time variables are injected into the binary:
- `Version`: Git tag or commit hash
- `Commit`: Git commit SHA
- `BuildDate`: ISO timestamp
- `DefaultConfigPath`: Default configuration location

## Documentation

- SDK Usage: `docs/sdk-usage.md`
- Advanced SDK: `docs/sdk-advanced.md`
- Amp CLI Integration: `docs/amp-cli-integration.md`
- Management API: Available at `https://help.router-for.me/management/api`

## Changes Folder

The `changes/` folder contains timestamped documentation and plans for feature implementations and modifications to the codebase. This serves as a historical record of changes made to the system, including implementation plans, architectural decisions, and feature specifications.

Current contents:
- `read-only-storage-feature-plan.md`: Plan for implementing the read-only storage feature with sync scheduler (Created: 2025-12-02, Last Modified: 2025-12-02)