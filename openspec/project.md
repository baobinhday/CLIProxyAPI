# Project Context

## Purpose
**CLIProxyAPI** is a proxy server designed to provide OpenAI/Gemini/Claude/Codex compatible API interfaces for CLI models.
It enables developers to:
- Use local or multi-account CLI access with major AI clients/SDKs (OpenAI, Gemini, Claude compatible).
- Perform load balancing across multiple accounts.
- Access models via OAuth login (Claude Code, OpenAI Codex, Qwen Code, iFlow).
- Support streaming, function calling, and multimodal inputs.
- Integrate with Amp CLI and IDE extensions.

## Tech Stack
- **Language**: Go 1.24+
- **Web Framework**: Gin (HTTP routing and middleware)
- **Database/Storage**:
  - PostgreSQL (`pgx` driver) for enterprise persistence.
  - Git-backed storage (`go-git`) for version control sync.
  - S3-compatible Object Storage (MinIO SDK) for cloud-native setups.
- **Authentication**: OAuth2 (`golang.org/x/oauth2`)
- **Real-time**: WebSockets (`gorilla/websocket`)
- **Logging**: Logrus (`sirupsen/logrus`)
- **Containerization**: Docker, Docker Compose

## Project Conventions

### Code Style
- Follow standard **Go conventions** (Effective Go).
- Use `go fmt` for formatting.
- Linting via `golangci-lint`.
- Dependencies managed via `go.mod`.

### Architecture Patterns
The project follows a modular, clean architecture:
- **`cmd/server/`**: Main entry point(s).
- **`internal/`**: Private application code.
  - `access/`: Auth & access control.
  - `config/`: Configuration loading (YAML + Env).
  - `store/`: Abstracted storage layer with multiple backends (Git, Postgres, S3).
  - `translator/`: Logic to convert between different AI Provider API formats.
- **`sdk/`**: Publicly importable SDK for embedding the proxy.
- **Hexagonal/Layered**: Separates transport (Gin), business logic (Translators/Proxy), and data access (Store).

### Testing Strategy
- **Unit Tests**: Co-located with source code (`*_test.go`).
- **Command**: `go test ./...`
- **Coverage**: `go test -cover ./...`
- **Mocking**: Mock external services/providers where necessary.

### Git Workflow
- **Standard Feature Branch Workflow**:
  - Create branch: `feature/your-feature-name`
  - Commit changes.
  - Open Pull Request.
- **Commit Messages**: Clear and descriptive (e.g., "Add support for X provider", "Fix bug in auth flow").

## Domain Context
- **Translators**: Critical component. The system must translate requests/responses between the client's expected format (often OpenAI-compatible) and the upstream provider's specific format (Gemini, Claude, etc.).
- **OAuth Providers**: The system acts as a central hub (router) managing tokens for various providers.
- **Load Balancing**: "Round-robin" strategy used to distribute usage across multiple accounts to avoid rate limits.
- **Read-Only Storage**: A mode where the server pulls config from a remote Git repo but doesn't write back, useful for immutable deployments.

## Important Constraints
- **Localhost-only Management**: Management APIs should generally be restricted to localhost or secured properly.
- **Environment Variables**: Heavily relied upon for configuration (`.env`, `config.yaml`).
- **Compatibility**: Must maintain compatibility with standard CLI tools that expect specific API behaviors (OpenAI signature).

## External Dependencies
- **AI Providers**: OpenAI, Google (Gemini), Anthropic (Claude), Qwen (Alibaba), iFlow.
- **Storage Services**: GitHub (for Git store), S3 Providers (AWS/MinIO), PostgreSQL.
- **Amp CLI**: Integrated support.
