# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an MCP (Model Context Protocol) server that provides a conversational interface to Argo CD. It allows AI assistants to query unhealthy applications and resources in Argo CD clusters.

**Key technologies:**
- Go 1.25
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) v1.4.1
- HTTP transport (no stdio transport)
- Argo CD v3 API

## Development Commands

All commands use [Task](https://taskfile.dev/). See `taskfile.yaml` for the full list.

### Building
```bash
# Build binary to $GOPATH/bin/argocd-mcp-server
task build

# Build container image
task build-image

# Build for Linux AMD64 specifically
task build-image-linux-amd64
```

### Testing
```bash
# Run unit tests
task test

# Run linting
task lint

# Run e2e tests (requires Podman and podman-compose)
task test-e2e
```

**E2E test infrastructure:**
- Uses `podman-compose` to orchestrate containers
- Spins up a mock Argo CD server (`test/e2e/argocd-server-mock/`)
- Tests multiple server configurations (standard, unreachable, stateless with load balancer)
- See `compose.yaml` for the full test setup

### Running Locally
```bash
# After building
argocd-mcp-server --argocd-url=<url> --argocd-token=<token> --debug=true --listen=127.0.0.1:8080

# With container
podman run -d --name argocd-mcp-server \
  -e ARGOCD_MCP_SERVER_LISTEN_HOST=0.0.0.0 \
  -e ARGOCD_URL=<url> \
  -e ARGOCD_TOKEN=<token> \
  -e ARGOCD_MCP_DEBUG=true \
  -p 8080:8080 argocd-mcp-server:latest
```

## Code Architecture

### Package Structure
```
cmd/
  start_server.go         # Cobra CLI command, flags, HTTP server setup
main.go                   # Entry point
internal/
  server/
    server.go             # MCP server initialization, middleware, tool/prompt registration
  argocd/
    argocd_client.go      # HTTP client for Argo CD API
    unhealthy_applications.go
    unhealthy_application_resources.go
```

### Key Architectural Patterns

**MCP Server Setup** (`internal/server/server.go`):
1. Creates MCP server with capabilities (tools, prompts)
2. Adds middleware (metrics, logging) via `mcp-common` package
3. Registers prompts and tools from `internal/argocd`
4. Supports stateless mode (disables ListChanged notifications for multi-replica deployments)

**HTTP Transport** (`cmd/start_server.go`):
- `/mcp` - MCP protocol endpoint (uses `mcp.NewStreamableHTTPHandler`)
- `/metrics` - Prometheus metrics
- `/health` - Health check endpoint

**Argo CD Client** (`internal/argocd/argocd_client.go`):
- Wraps `http.Client` with bearer token authentication
- Methods: `GetApplicationsWithContext()`, `GetApplicationWithContext()`
- Supports insecure TLS for development

**Tool Implementation Pattern**:
Each tool/prompt follows this pattern:
1. Define input/output types
2. Generate JSON schemas using `jsonschema-go`
3. Create `mcp.Tool` definition in `init()`
4. Implement handler function: `ToolHandle(logger, client) -> mcp.ToolHandlerFor[Input, Output]`

### Tool Naming Convention

Tools must follow this naming scheme (see `.cursor/skills/validating/SKILL.md`):
- Use snake_case with service prefix
- Format: `{service}_{action}_{resource}`
- Examples: `argocd_list_unhealthy_applications`, `argocd_list_unhealthy_application_resources`

### Stateless Mode

When `--stateless=true`:
- Disables `ListChanged` notifications for tools and prompts
- Required for multi-replica deployments or load balancing
- See `compose.yaml` for stateless configuration with nginx load balancer

## Testing Patterns

**Unit tests** (`internal/argocd/*_test.go`):
- Use `github.com/h2non/gock` to mock HTTP responses from Argo CD
- Test both success and error cases

**E2E tests** (`test/e2e/`):
- Test against containerized MCP server and mock Argo CD
- Verify different deployment scenarios (single instance, unreachable backend, load-balanced stateless)

## Dependencies

**Core MCP dependencies:**
- `github.com/modelcontextprotocol/go-sdk` - MCP protocol implementation
- `github.com/codeready-toolchain/mcp-common` - Shared middleware (metrics, logging)

**Argo CD integration:**
- `github.com/argoproj/argo-cd/v3` - Application types
- `github.com/argoproj/gitops-engine` - Health status types

## Environment Variables

When running the container, the server reads these environment variables:
- `ARGOCD_URL` - Argo CD server URL (required)
- `ARGOCD_TOKEN` - Bearer token for authentication (required)
- `ARGOCD_MCP_SERVER_LISTEN_HOST` - Listen host (default: 127.0.0.1)
- `ARGOCD_MCP_SERVER_LISTEN_PORT` - Listen port (default: 8080)
- `ARGOCD_MCP_SERVER_INSECURE` - Allow insecure TLS (default: false)
- `ARGOCD_MCP_SERVER_DEBUG` - Enable debug logging (default: false)
- `ARGOCD_MCP_SERVER_STATELESS` - Enable stateless mode (default: false)

See `cmd/start_server.go` for flag-to-environment mapping.
