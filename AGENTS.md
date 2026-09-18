# Drone MCP Server - Developer Guide for OpenCode

## Purpose
MCP server for Drone CI/CD that provides tools to query build information, repositories, and manage Drone resources via Model Context Protocol (MCP).

## Key Files
- `main.go` - Entry point: CLI flags, transport selection (stdio / Streamable HTTP), bearer authentication, HTTP hardening, tool registration
- `main_test.go` - Tests for the auth middleware, allowlists, log hygiene and tool registration
- `tool/` - All tool handlers (build.go, repo.go, secret.go, user.go, template.go, cron.go, resource.go)
- `tool/validate.go`, `tool/validate_args.go` - Input validation helpers and the per-tool `Validate()` implementations
- `go.mod` - Dependencies: drone-go v1.7.1, go-sdk v1.8.0 (requires Go 1.25.14+)
- `SECURITY.md` - Threat model, deployment guidance and vulnerability reporting process
- `Makefile` - Build system with targets: build, test, test-race, release, docker-build
- `README.md` - User documentation
- `.github/workflows/` - CI/CD pipelines (release.yml, docker.yml, mcp-publish.yaml)
- `Dockerfile` - Multi-stage Docker build (alpine-based, runs as non-root)

## Environment Variables
```bash
DRONE_SERVER=https://drone.example.com    # Required: Drone instance URL
DRONE_TOKEN=your_drone_token              # Required: personal access token
MCP_AUTH_TOKEN=your_mcp_token             # Required in HTTP mode (32+ random bytes)
MCP_AUTH_TOKEN_FILE=/run/secrets/token    # Optional: token file, wins over MCP_AUTH_TOKEN
MCP_ENABLE_WRITE_TOOLS=true               # Optional: register the 27 write/destructive tools
```
HTTP mode refuses to start without `MCP_AUTH_TOKEN` or `MCP_AUTH_TOKEN_FILE`; pass `--insecure-allow-anonymous` only for a throwaway local instance.

## Building & Running
```bash
# Build
make build          # Builds binary with version info injected
go build -o drone-mcp-server .  # Manual build

# Run modes
make run            # Stdio mode (default for MCP clients)
make run-http       # Streamable HTTP mode on localhost:8080 (requires MCP_AUTH_TOKEN)

# Direct execution
./drone-mcp-server                                     # Stdio mode
./drone-mcp-server --http --host 0.0.0.0 --port 8080   # HTTP mode (requires MCP_AUTH_TOKEN)
./drone-mcp-server --http --enable-write-tools         # HTTP mode with write tools registered
./drone-mcp-server --version                           # Show version info
```
The SSE transport is gone: `--sse` was renamed to `--http` (Streamable HTTP). The old SSE transport had been removed from the MCP specification and shipped without DNS rebinding protection.

## Tool Overview (45 tools)
The 18 read-only tools are always registered. The 27 write and destructive tools are registered only with `--enable-write-tools` (or `MCP_ENABLE_WRITE_TOOLS=true`), and every tool carries MCP annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`).
- **Repository management** (8): 3 read-only (`list_repos`, `get_repo`, `list_incomplete`) + 5 write (`enable_repo`, `disable_repo`, `repair_repo`, `chown_repo`, `sync_repos`)
- **Build management** (11): 4 read-only (`list_builds`, `get_build`, `get_build_last`, `get_build_logs`) + 7 write (`restart_build`, `cancel_build`, `promote_build`, `rollback_build`, `approve_build`, `decline_build`, `create_build`)
- **Cron job management** (5): 2 read-only (`list_crons`, `get_cron`) + 3 write (`create_cron`, `delete_cron`, `execute_cron`)
- **Secret management** (10): 4 read-only (repository and organization list/get) + 6 write (create/update/delete for both)
- **User management** (6): 3 read-only (`get_self`, `list_users`, `get_user`) + 3 write (`create_user`, `update_user`, `delete_user`)
- **Template management** (5): 2 read-only (`list_templates`, `get_template`) + 3 write (`create_template`, `update_template`, `delete_template`)

## Resource Access
- Resource URI: `drone://builds/{owner}/{repo}/{build}`
- Handled in `tool/resource.go` - provides build details as resource

## Testing & Validation
```bash
# Unit and integration tests (no Drone instance required)
make test                            # Run Go tests
make test-race                       # Run Go tests with the race detector
make test-coverage                   # Generate coverage report

# End-to-end security smoke test: starts the real binary on a loopback port and
# asserts auth, Host/Origin validation, request limits, tool surface and log
# hygiene. Exits non-zero on failure and runs in CI.
make smoke
BIN=./drone-mcp-server BASE_PORT=18300 ./scripts/security-smoke-test.sh

# Manual verification
export DRONE_SERVER=https://ci.yusiwen.cn
export DRONE_TOKEN=...
MCP_AUTH_TOKEN=dev-token ./drone-mcp-server --http   # Should start without errors

# Probe the HTTP endpoint: unauthenticated requests must return 401
curl -i -X POST -H 'Content-Type: application/json' \
     -H 'Accept: application/json, text/event-stream' \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
     http://localhost:8080/
```

## Common Errors & Fixes
1. **"DRONE_SERVER and DRONE_TOKEN environment variables must be set"** - Set both env vars
2. **"refusing to start HTTP mode without MCP_AUTH_TOKEN"** - Set `MCP_AUTH_TOKEN` (or `MCP_AUTH_TOKEN_FILE`); do not reach for `--insecure-allow-anonymous` outside a throwaway local instance
3. **HTTP mode returns 403 behind a reverse proxy** - The transport rejects a public `Host` header on a loopback connection. Either keep the loopback `Host` in the proxy config, or set `--allowed-hosts <public-host>`, which replaces the loopback heuristic (no need for `--localhost-protection=false`)
4. **Only 18 tools are visible** - Read-only mode is the default; add `--enable-write-tools` (or `MCP_ENABLE_WRITE_TOOLS=true`) to register the 27 write tools
5. **"invalid arguments for <tool>"** - Input validation rejected the value; identifiers may only contain letters, digits, dots, underscores and hyphens, must start with a letter or digit, and branches may additionally contain slashes
6. **Build errors with drone-go** - Check Line struct fields: `Number` and `Message` (not `Pos` and `Out`)
7. **Version info not showing** - Use `make build` to inject ldflags, or manually add `-ldflags="-X main.buildVersion=..."`

## Code Style & Conventions
- **No hardcoded secrets** - Always use environment variables or `MCP_AUTH_TOKEN_FILE`
- **Every tool argument struct implements `Validate()`** - `registerTool` in `main.go` only accepts argument types satisfying the `validated` interface and runs `Validate()` before the handler. Identifiers that reach a Drone API path must go through the `tool/validate.go` helpers
- **Adding a tool** - Implement the handler in `tool/`, add the args struct plus its `Validate()` in `tool/validate_args.go`, then register it in `registerTools` with `readOnlyTool()` or `writeTool(destructive, idempotent)`
- **Tool organization** - Each tool category in separate file under `tool/`
- **Error handling** - Return MCP tool errors with descriptive messages; never include secrets in errors or logs
- **Logging** - Tool arguments are never logged; sanitize any request-derived value with `sanitizeLog` before logging it
- **MCP SDK patterns** - Follow examples in `github.com/modelcontextprotocol/go-sdk`
- **Go version** - 1.25.14+ (see go.mod; required by go-sdk v1.8.0, and earlier 1.25 patches fail the govulncheck gate)
- **Platform support** - Build for 5 platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- **Docker multi-arch** - Supports linux/amd64 and linux/arm64

## Development Tools & Package Managers

When running CLI tools during OpenCode sessions, use the following package managers to ensure isolation and version consistency:

### Python Tools
- Use `uvx` (uv tool runner) for Python-based tools
- Example: `uvx black --check .` (format checking)
- Example: `uvx ruff check .` (linting)  
- Example: `uvx pytest tests/` (running tests)

### Node.js Tools
- Use `npx` (npm package runner) for Node.js-based tools
- Example: `npx prettier --check .` (code formatting)
- Example: `npx eslint .` (linting)
- Example: `npx jest` (testing)

### Go Tools
- Use `go run` for single-file execution: `go run ./cmd/tool.go`
- Pin tool versions when installing: `go install github.com/xxx/tool@v1.2.3` (avoid `@latest` in scripts and CI)

### Rationale
- Avoids global package installation conflicts
- Ensures consistent tool versions across sessions
- Follows modern development best practices

## Release Process
```bash
make release          # Builds binaries for all platforms, creates archives in releases/
# GitHub Actions releases on tag push (see .github/workflows/release.yml)
# Docker images are built and pushed to GHCR on tag push (see .github/workflows/docker.yml)
# The MCP registry entry is published after the Docker workflow completes (mcp-publish.yaml)
```

## Quick Start for New Features
1. Add the tool handler to the appropriate file in `tool/`
2. Add the args struct and its `Validate()` implementation in `tool/validate_args.go`
3. Register the tool in `registerTools` (`main.go`) with the correct annotations
4. Add a test (see `main_test.go`, `tool/validate_test.go`)
5. Update README.md with the new tool documentation
6. Run `make test` and `make check` (fmt, vet, lint) before committing

## Notes for OpenCode Sessions
- This project has been tested with actual Drone server (ci.yusiwen.cn)
- Authentication, allowlists and HTTP hardening live in `main.go`; the HTTP transport is the SDK's `StreamableHTTPHandler`
- Read-only is the default: write tools are hidden unless explicitly enabled
- Version info injected via ldflags in Makefile
- 45 tools are implemented (18 read-only, 27 write/destructive)
- Tool calls are automatically logged via MCP middleware (receiving middleware); tool arguments are never logged
- `go vet ./...` and `go test ./...` must pass before committing
- Project structure is stable - follow existing patterns