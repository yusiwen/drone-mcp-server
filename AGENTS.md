# Drone MCP Server - Developer Guide for OpenCode

## Purpose
MCP server for Drone CI/CD that provides tools to query build information, repositories, and manage Drone resources via Model Context Protocol (MCP).

## Key Files
- `main.go` - Entry point with CLI args, SSE auth middleware, version info
- `tool/` - All tool handlers (build.go, repo.go, secret.go, user.go, template.go, cron.go, resource.go)
- `go.mod` - Dependencies: drone-go v1.7.1, mcp-sdk v1.5.0
- `Makefile` - Build system with targets: build, test, release, docker-build
- `README.md` - User documentation
- `.github/workflows/` - CI/CD pipelines (release.yml, docker.yml)
- `Dockerfile` - Multi-stage Docker build (alpine-based)

## Environment Variables (Required)
```bash
DRONE_SERVER=https://drone.example.com    # Drone instance URL
DRONE_TOKEN=your_drone_token             # Personal access token
MCP_AUTH_TOKEN=your_sse_token            # Optional: SSE Bearer token authentication
```

## Building & Running
```bash
# Build
make build          # Builds binary with version info injected
go build -o drone-mcp-server .  # Manual build

# Run modes
make run            # Stdio mode (default for MCP clients)
make run-sse        # SSE mode on localhost:8080

# Direct execution
./drone-mcp-server                     # Stdio mode
./drone-mcp-server --sse --host 0.0.0.0 --port 8080  # SSE HTTP mode
./drone-mcp-server --version           # Show version info
```

## Tool Overview (47 tools)
- **Repository management** (8): `list_repos`, `get_repo`, `enable_repo`, `disable_repo`, `repair_repo`, `chown_repo`, `sync_repos`, `list_incomplete`
- **Build management** (11): `list_builds`, `get_build`, `get_build_last`, `get_build_logs`, `restart_build`, `cancel_build`, `promote_build`, `rollback_build`, `approve_build`, `decline_build`, `create_build`
- **Cron job management** (5): `list_crons`, `get_cron`, `create_cron`, `delete_cron`, `execute_cron`
- **Secret management** (10): Repository and organization secrets (list/get/create/update/delete for both)
- **User management** (6): `get_self`, `list_users`, `get_user`, `create_user`, `update_user`, `delete_user`
- **Template management** (5): `list_templates`, `get_template`, `create_template`, `update_template`, `delete_template`

## Resource Access
- Resource URI: `drone://builds/{owner}/{repo}/{build}`
- Handled in `tool/resource.go` - provides build details as resource

## Testing & Validation
```bash
# Set test environment
source test_env.sh                    # Loads sample env vars

# Run tests
make test                            # Run Go tests
make test-coverage                   # Generate coverage report

# Manual verification
export DRONE_SERVER=https://ci.yusiwen.cn
export DRONE_TOKEN=...
./drone-mcp-server                   # Should start without errors

# Check connectivity
curl -H "Authorization: Bearer $MCP_AUTH_TOKEN" http://localhost:8080/  # SSE endpoint
```

## Common Errors & Fixes
1. **"missing Drone client configuration"** - Set `DRONE_SERVER` and `DRONE_TOKEN` env vars
2. **SSE authentication failures** - Ensure `MCP_AUTH_TOKEN` matches client's Bearer token
3. **Build errors with drone-go** - Check Line struct fields: `Number` and `Message` (not `Pos` and `Out`)
4. **Version info not showing** - Use `make build` to inject ldflags, or manually add `-ldflags="-X main.buildVersion=..."`

## Code Style & Conventions
- **No hardcoded secrets** - Always use environment variables
- **Tool organization** - Each tool category in separate file under `tool/`
- **Error handling** - Return MCP tool errors with descriptive messages
- **MCP SDK patterns** - Follow examples in `github.com/modelcontextprotocol/go-sdk`
- **Go version** - 1.21+ (see go.mod)
- **Platform support** - Build for 5 platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- **Docker multi-arch** - Supports linux/amd64 and linux/arm64

## Release Process
```bash
make release          # Builds binaries for all platforms, creates archives in releases/
# GitHub Actions automatically releases on tag push (see .github/workflows/release.yml)
# Docker images built and pushed to GHCR on push to main (see .github/workflows/docker.yml)
```

## Quick Start for New Features
1. Add tool handler to appropriate file in `tool/` directory
2. Register tool in `main.go` server initialization
3. Test with `make run-sse` and connect via MCP client
4. Update README.md with new tool documentation
5. Run `make check` (fmt, vet, lint) before committing

## Notes for OpenCode Sessions
- This project has been tested with actual Drone server (ci.yusiwen.cn)
- SSE authentication middleware is custom (not provided by MCP SDK)
- Version info injected via ldflags in Makefile
- All 47 tools are implemented and functional
- Project structure is stable - follow existing patterns