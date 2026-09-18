# Drone MCP Server

A Model Context Protocol (MCP) server for interacting with Drone CI/CD. This server provides tools and resources to query build information, repositories, and more from your Drone instance.

## Features

- **45 tools** covering repositories, builds, cron jobs, secrets, users and templates
- **Read-only by default**: the 27 write and destructive tools are only registered with `--enable-write-tools`
- **Streamable HTTP transport** with mandatory bearer authentication, DNS rebinding protection and cross-origin protection
- **Validated inputs**: repository identifiers, names, branches and build numbers are checked before they reach the Drone API
- **Resource access**: build details via `drone://builds/{owner}/{repo}/{build}`

## Installation

```bash
go mod tidy
go build -o drone-mcp-server .
```

## Configuration

### Environment variables

| Variable | Required | Description |
| :--- | :--- | :--- |
| `DRONE_SERVER` | yes | Base URL of the Drone instance, e.g. `https://drone.example.com` |
| `DRONE_TOKEN` | yes | Drone personal access token, sent as `Authorization: Bearer <token>` on every Drone API call |
| `MCP_AUTH_TOKEN` | HTTP mode | Bearer token that clients must present. HTTP mode refuses to start without it |
| `MCP_AUTH_TOKEN_FILE` | no | File containing the bearer token (Docker/Kubernetes secrets). Takes precedence over `MCP_AUTH_TOKEN` |
| `MCP_ENABLE_WRITE_TOOLS` | no | `true` registers the write and destructive tools, equivalent to `--enable-write-tools` |

```bash
export DRONE_SERVER=https://drone.example.com
export DRONE_TOKEN=your_drone_token
export MCP_AUTH_TOKEN="$(openssl rand -base64 32)"   # HTTP mode only
```

Grant the smallest Drone permission set that covers the tools you expose. The read-only tool set only needs read access to repositories and builds, while write tools such as `delete_user`, `create_org_secret` or `chown_repo` require administrator permissions.

## Security

This server is a privileged proxy: whatever it can reach, anyone who can talk to it can reach too. The defaults reflect that.

- **HTTP mode fails closed.** Without `MCP_AUTH_TOKEN` (or `MCP_AUTH_TOKEN_FILE`) the server refuses to start in HTTP mode. `--insecure-allow-anonymous` overrides this and logs a warning.
- **Read-only by default.** Only the 18 read-only tools are registered unless write access is requested explicitly. Every tool is annotated with the MCP hints `readOnlyHint`, `destructiveHint` and `idempotentHint` so clients can gate writes themselves.
- **Streamable HTTP transport.** The legacy SSE transport was removed from the MCP specification and shipped without DNS rebinding or cross-origin protection. This server uses the Go SDK's `StreamableHTTPHandler`, which rejects requests that arrive on a loopback address with a non-loopback `Host` header and rejects cross-site browser requests.
- **Optional allowlists.** `--allowed-hosts` and `--allowed-origins` add explicit `Host` and `Origin` allowlists on top of the transport defaults.
- **Bearer token handling.** Tokens are compared in constant time over SHA-256 digests, and repeated failures from one address are throttled (10 per minute, then `429`).
- **Input validation.** Repository identifiers, secret and template names, branches and build numbers are validated before reaching the Drone API client, because drone-go assembles request paths by string formatting.
- **Log hygiene.** `X-Forwarded-For` / `X-Real-IP` are ignored unless `--trust-proxy` is set, and control characters are stripped from logged values so request data cannot forge log lines.
- **Bounded requests.** Request bodies are capped (1 MiB by default), idle sessions expire, and header read timeouts plus silent-connection timeouts limit slow-client abuse.

### Deployment checklist

1. Terminate TLS in front of the server (the binary speaks plain HTTP) and never send the bearer token over an untrusted network.
2. Keep the loopback default binding and put a reverse proxy in front, or bind to an interface only your MCP clients can reach.
3. Generate a high-entropy `MCP_AUTH_TOKEN` (32+ random bytes) and inject it as a platform secret rather than on the command line.
4. Leave write tools disabled unless the workflow needs them.
5. Set `--trust-proxy` only when a proxy you control rewrites `X-Forwarded-For` / `X-Real-IP`; otherwise the logged client address is attacker controlled.
6. Read [SECURITY.md](SECURITY.md) before exposing the server beyond localhost.

## Usage

### As an MCP server

Add the server to your MCP client configuration (e.g., Claude Desktop):

```json
{
  "mcpServers": {
    "drone": {
      "command": "/path/to/drone-mcp-server",
      "env": {
        "DRONE_SERVER": "https://drone.example.com",
        "DRONE_TOKEN": "your_token"
      }
    }
  }
}
```

### Direct execution

You can run the server directly for testing:

```bash
# Stdio mode (default) - for local MCP clients
./drone-mcp-server

# Streamable HTTP mode - requires MCP_AUTH_TOKEN
export MCP_AUTH_TOKEN="$(openssl rand -base64 32)"
./drone-mcp-server --http --host localhost --port 8080
```

#### Transport Modes

1. **Stdio mode (default)**: Communicates via stdin/stdout using the MCP protocol. Suitable for local integration with MCP clients; no network exposure and no authentication required.

2. **Streamable HTTP mode** (`--http`): A single endpoint that accepts `POST` (client to server), `GET` (server to client stream) and `DELETE` (session teardown).

   > **Upgrading from earlier releases:** the old `--sse` flag and the SSE transport are gone. Use `--http` and a client that speaks Streamable HTTP.

   ```bash
   # Local only, read-only tools
   MCP_AUTH_TOKEN=... ./drone-mcp-server --http

   # Behind a TLS terminating proxy, write tools enabled
   MCP_AUTH_TOKEN=... ./drone-mcp-server --http --host 127.0.0.1 --port 8080 \
       --enable-write-tools --allowed-hosts mcp.example.com
   ```

   Clients connect to `http://localhost:8080/` by default and must send the header:

   ```
   Authorization: Bearer <MCP_AUTH_TOKEN>
   ```

   **Behind a reverse proxy**: the transport rejects a public `Host` header on a loopback connection with `403` as part of its DNS rebinding protection. Either keep the loopback `Host` header (`proxy_set_header Host 127.0.0.1:8080;`), or hand Host validation to the allowlist with a single flag:

   ```bash
   ./drone-mcp-server --http --allowed-hosts mcp.example.com
   ```

   Setting `--allowed-hosts` replaces the loopback heuristic, so you do not need `--localhost-protection=false`. If you pass `--localhost-protection=true` explicitly, both checks are applied and the public `Host` is rejected again.

#### Command line flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--http` | `false` | Use the Streamable HTTP transport |
| `--host` / `--port` | `localhost` / `8080` | Listen address (HTTP mode only) |
| `--path` | `/` | Path of the MCP endpoint (HTTP mode only) |
| `--enable-write-tools` | `false` | Register the 27 write and destructive tools |
| `--insecure-allow-anonymous` | `false` | Allow HTTP mode without `MCP_AUTH_TOKEN` |
| `--trust-proxy` | `false` | Trust `X-Forwarded-For` / `X-Real-IP` from a reverse proxy |
| `--allowed-hosts` | *(empty)* | Comma-separated `Host` allowlist; replaces the loopback heuristic |
| `--allowed-origins` | *(empty)* | Comma-separated browser `Origin` allowlist |
| `--localhost-protection` | `true` | Reject loopback connections carrying a non-loopback `Host` header; set explicitly together with `--allowed-hosts` to stack both checks |
| `--session-timeout` | `30m` | Idle MCP session timeout (HTTP mode only) |
| `--max-request-bytes` | `1048576` | Maximum MCP request body size |
| `--version` | | Print version information and exit |

## Available Tools

The server implements 45 tools. In the default read-only mode only the 18 read-only tools are registered; the remaining 27 are write or destructive tools and require `--enable-write-tools` (or `MCP_ENABLE_WRITE_TOOLS=true`).

**Write and destructive tools:** `enable_repo`, `disable_repo`, `repair_repo`, `chown_repo`, `sync_repos`, `create_cron`, `delete_cron`, `execute_cron`, `create_secret`, `update_secret`, `delete_secret`, `create_org_secret`, `update_org_secret`, `delete_org_secret`, `create_user`, `update_user`, `delete_user`, `create_template`, `update_template`, `delete_template`, `restart_build`, `cancel_build`, `promote_build`, `rollback_build`, `approve_build`, `decline_build`, `create_build`.

The destructive ones (deletions, disables, cancellations, promotions, rollbacks, ownership changes) are advertised with `destructiveHint: true`; the rest of the write tools with `destructiveHint: false`. All 18 read-only tools are advertised with `readOnlyHint: true`.

### Repository Management

#### `list_repos`
Lists all repositories in your Drone instance.

#### `get_repo`
Get repository details.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `enable_repo`
Enable a repository.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `disable_repo`
Disable a repository.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `repair_repo`
Repair a repository.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `chown_repo`
Change repository ownership.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `sync_repos`
Synchronize repository list.

#### `list_incomplete`
List repositories with incomplete builds.

### Build Management

#### `list_builds`
Lists builds for a specific repository.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `get_build`
Get detailed information about a specific build.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number

#### `get_build_last`
Get the last build for a repository (optionally by branch).

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `branch` (string, optional): Branch name

#### `get_build_logs`
Get logs for a specific build stage and step.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number
- `stage` (number): Stage number
- `step` (number): Step number

#### `restart_build`
Restart a build (optionally with parameters).

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number
- `params` (object, optional): Build parameters

#### `cancel_build`
Cancel a running build.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number

#### `promote_build`
Promote a build to a target environment.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number
- `target` (string): Target environment
- `params` (object, optional): Promotion parameters

#### `rollback_build`
Rollback a deployment to a previous build.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number
- `target` (string): Target environment
- `params` (object, optional): Rollback parameters

#### `approve_build`
Approve a build stage (for gated deployments).

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number
- `stage` (number): Stage number

#### `decline_build`
Decline a build stage (for gated deployments).

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number
- `stage` (number): Stage number

#### `create_build`
Create a new build from a commit or branch.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `commit` (string, optional): Commit SHA
- `branch` (string, optional): Branch name
- `params` (object, optional): Build parameters

### Cron Job Management

#### `list_crons`
List cron jobs for a repository.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `get_cron`
Get cron job details.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `cron` (string): Cron job name

#### `create_cron`
Create a new cron job.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `name` (string): Cron job name
- `expr` (string): Cron expression
- `branch` (string): Branch name
- `disable` (boolean, optional): Disable the cron job

#### `delete_cron`
Delete a cron job.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `cron` (string): Cron job name

#### `execute_cron`
Execute a cron job immediately.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `cron` (string): Cron job name

### Secret Management

#### `list_secrets`
List repository secrets.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

#### `get_secret`
Get repository secret details.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `name` (string): Secret name

#### `create_secret`
Create a repository secret.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `name` (string): Secret name
- `value` (string): Secret value
- `pull_request` (boolean, optional): Allow in pull requests
- `pull_request_push` (boolean, optional): Allow in pull request push events

#### `update_secret`
Update a repository secret.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `name` (string): Secret name
- `value` (string): Secret value
- `pull_request` (boolean, optional): Allow in pull requests
- `pull_request_push` (boolean, optional): Allow in pull request push events

#### `delete_secret`
Delete a repository secret.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `name` (string): Secret name

#### Organization Secrets

#### `list_org_secrets`
List organization secrets.

**Arguments:**
- `namespace` (string): Organization namespace

#### `get_org_secret`
Get organization secret details.

**Arguments:**
- `namespace` (string): Organization namespace
- `name` (string): Secret name

#### `create_org_secret`
Create an organization secret.

**Arguments:**
- `namespace` (string): Organization namespace
- `name` (string): Secret name
- `value` (string): Secret value
- `pull_request` (boolean, optional): Allow in pull requests
- `pull_request_push` (boolean, optional): Allow in pull request push events

#### `update_org_secret`
Update an organization secret.

**Arguments:**
- `namespace` (string): Organization namespace
- `name` (string): Secret name
- `value` (string): Secret value
- `pull_request` (boolean, optional): Allow in pull requests
- `pull_request_push` (boolean, optional): Allow in pull request push events

#### `delete_org_secret`
Delete an organization secret.

**Arguments:**
- `namespace` (string): Organization namespace
- `name` (string): Secret name

### User Management

#### `get_self`
Get current authenticated user.

#### `list_users`
List all users.

#### `get_user`
Get user details.

**Arguments:**
- `login` (string): User login name

#### `create_user`
Create a new user.

**Arguments:**
- `login` (string): User login name
- `email` (string, optional): User email
- `admin` (boolean, optional): Admin privileges
- `active` (boolean, optional): Active status
- `token` (string, optional): User token

#### `update_user`
Update a user.

**Arguments:**
- `login` (string): User login name
- `admin` (boolean, optional): Admin privileges
- `active` (boolean, optional): Active status

#### `delete_user`
Delete a user.

**Arguments:**
- `login` (string): User login name

### Template Management

#### `list_templates`
List templates (optionally by namespace).

**Arguments:**
- `namespace` (string, optional): Template namespace

#### `get_template`
Get template details and data.

**Arguments:**
- `namespace` (string): Template namespace
- `name` (string): Template name

#### `create_template`
Create a new template.

**Arguments:**
- `namespace` (string): Template namespace
- `name` (string): Template name
- `data` (string): Template data (YAML)

#### `update_template`
Update a template.

**Arguments:**
- `namespace` (string): Template namespace
- `name` (string): Template name
- `data` (string): Template data (YAML)

#### `delete_template`
Delete a template.

**Arguments:**
- `namespace` (string): Template namespace
- `name` (string): Template name

## Resources

### Build details resource

Access build details via resource URI: `drone://builds/{owner}/{repo}/{build}`

**Example:**
```
Read resource: drone://builds/owner1/repo1/123
```

## Docker

A multi-architecture Docker image is available on GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/yusiwen/drone-mcp-server:latest

# Run in stdio mode (for local MCP clients)
docker run --rm -i \
           -e DRONE_SERVER=https://drone.example.com \
           -e DRONE_TOKEN=your_token \
           ghcr.io/yusiwen/drone-mcp-server

# Run in Streamable HTTP mode: read-only, authenticated, published on loopback
docker run --rm \
           -e DRONE_SERVER=https://drone.example.com \
           -e DRONE_TOKEN=your_token \
           -e MCP_AUTH_TOKEN=your_mcp_token \
           -p 127.0.0.1:8080:8080 \
           ghcr.io/yusiwen/drone-mcp-server --http --host 0.0.0.0

# Add --enable-write-tools when the workflow must mutate Drone
docker run --rm \
           -e DRONE_SERVER=https://drone.example.com \
           -e DRONE_TOKEN=your_token \
           -e MCP_AUTH_TOKEN=your_mcp_token \
           -p 127.0.0.1:8080:8080 \
           ghcr.io/yusiwen/drone-mcp-server --http --host 0.0.0.0 --enable-write-tools
```

`--host 0.0.0.0` is required inside a container so that published ports work, which is exactly why the bearer token is mandatory. Publish the port on `127.0.0.1` unless a TLS terminating proxy sits in front.

Prefer a mounted secret over an environment variable:

```bash
docker run --rm \
           -v /run/secrets/mcp_token:/run/secrets/mcp_token:ro \
           -e MCP_AUTH_TOKEN_FILE=/run/secrets/mcp_token \
           -e DRONE_SERVER=https://drone.example.com \
           -e DRONE_TOKEN=your_token \
           -p 127.0.0.1:8080:8080 \
           ghcr.io/yusiwen/drone-mcp-server --http --host 0.0.0.0
```

### Docker Compose Example

```yaml
services:
  drone-mcp-server:
    image: ghcr.io/yusiwen/drone-mcp-server:latest
    environment:
      DRONE_SERVER: https://drone.example.com
      DRONE_TOKEN: ${DRONE_TOKEN:?DRONE_TOKEN is required}
      MCP_AUTH_TOKEN: ${MCP_AUTH_TOKEN:?MCP_AUTH_TOKEN is required}
      # MCP_ENABLE_WRITE_TOOLS: "true"   # only when the workflow needs writes
    ports:
      - "127.0.0.1:8080:8080"
    command: ["--http", "--host", "0.0.0.0"]
    restart: unless-stopped
    read_only: true
    security_opt:
      - no-new-privileges:true
```

## Releases

Pre-built binaries are available for Linux (x64, arm64), macOS (x64, arm64), and Windows (x64) in the [GitHub Releases](https://github.com/yusiwen/drone-mcp-server/releases).

### Version Information

The binary includes version information:

```bash
./drone-mcp-server --version
```

Output example:
```
drone-mcp-server
Version: v1.0.0
Commit: abc123
Build date: 2024-01-01T00:00:00Z
Go version: go1.25.1
```

## Project Structure

```
.
├── main.go              # Entry point: CLI flags, transports, auth, HTTP hardening, tool registration
├── main_test.go         # Auth middleware, allowlists, log hygiene and tool registration tests
├── tool/                # Tool handlers module
│   ├── build.go         # Build-related tools (list_builds, get_build, restart_build, etc.)
│   ├── repo.go          # Repository-related tools (list_repos, enable_repo, disable_repo, etc.)
│   ├── resource.go      # Resource handling (drone://builds/...)
│   ├── cron.go          # Cron job management tools
│   ├── secret.go        # Secret management tools
│   ├── user.go          # User management tools
│   ├── template.go      # Template management tools
│   ├── validate.go      # Input validation helpers used by every tool
│   ├── validate_args.go # Validate implementation for each tool argument struct
│   └── validate_test.go # Validation tests, including injection attempts
├── SECURITY.md          # Threat model, deployment guidance and reporting process
├── server.json          # MCP registry metadata
├── test_env.sh          # Manual smoke test script (uses environment variables)
├── test_mcp.go          # In-memory MCP integration test (build tag: test)
└── README.md
```

## Development

### Dependencies

- Go 1.25.14+ (the toolchain is pinned by `go.mod`; earlier Go 1.25 patch releases carry standard library vulnerabilities that fail the `govulncheck` gate)
- [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) - MCP SDK (Streamable HTTP transport)
- [github.com/drone/drone-go](https://github.com/drone/drone-go) - Drone API client

### Building

```bash
make build           # build with version info injected through ldflags
go build -o drone-mcp-server .
```

### Testing

```bash
make test                                  # unit tests
make test-race                             # with the race detector
make smoke                                 # end-to-end security smoke test
make vuln                                  # govulncheck (module + standard library)
go vet ./...
```

The unit tests cover the authentication middleware (including constant time comparison and failure throttling), the `Host` policy, log sanitization, client address handling, input validation and the read-only tool registration. They need no Drone instance.

### Verifying the security hardening

`scripts/security-smoke-test.sh` starts the real binary on a loopback port and asserts the behaviour that matters, without contacting any Drone instance:

```bash
./scripts/security-smoke-test.sh          # builds the binary first
BIN=./drone-mcp-server ./scripts/security-smoke-test.sh
```

It checks that HTTP mode refuses to start without a token, that missing/wrong credentials, wrong `Content-Type`, oversized bodies, forged `Host` headers and cross-site `Origin` are rejected with `401`/`401`/`415`/`413`/`403`/`403`, that the default tool list is exactly 18 read-only tools and that hidden write tools are not callable, that path traversal in tool arguments is refused, that log injection and `X-Forwarded-For` spoofing are neutralised, that `--allowed-hosts` replaces the loopback heuristic, that write tools appear only with `--enable-write-tools`, that `--sse` is gone, and that SIGTERM shuts the server down. It exits non-zero on the first failure set and runs in CI, so it doubles as a regression gate.

### Manual verification against a real Drone

Use a read-only token first and confirm the tools answer:

```bash
export DRONE_SERVER=https://ci.example.com
export DRONE_TOKEN=<read-only token>
export MCP_AUTH_TOKEN="$(openssl rand -base64 32)"
./drone-mcp-server --http --host 127.0.0.1 --port 8080
```

Then point an MCP client at it. Stdio mode:

```json
{
  "mcpServers": {
    "drone": {
      "command": "/path/to/drone-mcp-server",
      "env": { "DRONE_SERVER": "https://ci.example.com", "DRONE_TOKEN": "..." }
    }
  }
}
```

Streamable HTTP mode (most clients accept a `headers` block; `npx @modelcontextprotocol/inspector` works too, pick "Streamable HTTP" and add the same header):

```json
{
  "mcpServers": {
    "drone": {
      "url": "http://127.0.0.1:8080/",
      "headers": { "Authorization": "Bearer <MCP_AUTH_TOKEN>" }
    }
  }
}
```

Suggested checks against a real instance: `get_self` (confirms the token works), `list_repos`, `get_build_last`, `get_build_logs` (exercises untrusted content in the model context), then `get_repo` with `owner: "../users"` to confirm validation refuses it. Run write tools against a disposable repository only.

### Running locally

```bash
export DRONE_SERVER=https://drone.example.com
export DRONE_TOKEN=...

# Read-only stdio server
./drone-mcp-server

# HTTP mode with authentication and write tools
MCP_AUTH_TOKEN=dev-token ./drone-mcp-server --http --enable-write-tools
```

## License

MIT