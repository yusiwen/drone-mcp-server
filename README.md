# Drone MCP Server

A Model Context Protocol (MCP) server for interacting with Drone CI/CD. This server provides tools and resources to query build information, repositories, and more from your Drone instance.

## Features

- **List repositories**: Get all repositories in your Drone instance
- **List builds**: List builds for a specific repository
- **Get build details**: Retrieve detailed information about a specific build
- **Resource access**: Access build details via resource URIs

## Installation

```bash
go mod tidy
go build -o drone-mcp-server .
```

## Configuration

Set the following environment variables:

```bash
export DRONE_SERVER=https://drone.example.com
export DRONE_TOKEN=your_drone_token
# Optional: For SSE transport authentication
export MCP_AUTH_TOKEN=your_sse_auth_token
```

The `DRONE_TOKEN` should be a personal access token with appropriate permissions to read repositories and builds.

**SSE Authentication**: When using SSE transport, you can optionally set `MCP_AUTH_TOKEN` to require Bearer token authentication. Clients must include `Authorization: Bearer <token>` header in their requests. If not set, SSE endpoints will be publicly accessible (use with caution in production).

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
# Stdio mode (default)
./drone-mcp-server

# SSE HTTP mode
./drone-mcp-server --sse --host localhost --port 8080
```

#### Transport Modes

1. **Stdio mode (default)**: Communicates via stdin/stdout using the MCP protocol. Suitable for local integration with MCP clients.

2. **SSE HTTP mode**: Uses Server-Sent Events (SSE) over HTTP. Suitable for remote access or testing.

   ```bash
   # Without authentication (public access)
   ./drone-mcp-server --sse --host 0.0.0.0 --port 8080
   
   # With authentication (recommended for production)
    export MCP_AUTH_TOKEN=your-secret-token
   ./drone-mcp-server --sse --host 0.0.0.0 --port 8080
   ```
   
   The server will be available at `http://localhost:8080/` for SSE connections.
   
    **Authentication**: If `MCP_AUTH_TOKEN` is set, clients must include the header:
   ```
   Authorization: Bearer your-secret-token
   ```

## Available Tools

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

## Project Structure

```
.
├── main.go              # Main entry point, handles command line arguments and server startup
├── tool/                # Tool handlers module
│   ├── build.go         # Build-related tools (list_builds, get_build, restart_build, etc.)
│   ├── repo.go          # Repository-related tools (list_repos, enable_repo, disable_repo, etc.)
│   ├── resource.go      # Resource handling (drone://builds/...)
│   ├── cron.go          # Cron job management tools
│   ├── secret.go        # Secret management tools
│   ├── user.go          # User management tools
│   └── template.go      # Template management tools
├── test_env.sh          # Test script (uses environment variables)
├── test_mcp.go          # MCP integration test
└── README.md
```

## Development

### Dependencies

- Go 1.21+
- [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) - MCP SDK
- [github.com/drone/drone-go](https://github.com/drone/drone-go) - Drone API client

### Building

```bash
go build -o drone-mcp-server .
```

### Testing

Set environment variables and run the server:

```bash
export DRONE_SERVER=...
export DRONE_TOKEN=...
./drone-mcp-server
```

## License

MIT