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
export DRONE_SSE_TOKEN=your_sse_auth_token
```

The `DRONE_TOKEN` should be a personal access token with appropriate permissions to read repositories and builds.

**SSE Authentication**: When using SSE transport, you can optionally set `DRONE_SSE_TOKEN` to require Bearer token authentication. Clients must include `Authorization: Bearer <token>` header in their requests. If not set, SSE endpoints will be publicly accessible (use with caution in production).

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
   export DRONE_SSE_TOKEN=your-secret-token
   ./drone-mcp-server --sse --host 0.0.0.0 --port 8080
   ```
   
   The server will be available at `http://localhost:8080/` for SSE connections.
   
   **Authentication**: If `DRONE_SSE_TOKEN` is set, clients must include the header:
   ```
   Authorization: Bearer your-secret-token
   ```

## Available Tools

### `list_repos`

Lists all repositories in your Drone instance.

**Example usage:**
```
list_repos()
```

**Response:**
```
owner1/repo1
owner2/repo2
```

### `list_builds`

Lists builds for a specific repository.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name

**Example usage:**
```
list_builds(owner="owner1", repo="repo1")
```

**Response:**
```
#123 success refs/heads/main
#122 failure refs/heads/feature
```

### `get_build`

Get detailed information about a specific build.

**Arguments:**
- `owner` (string): Repository owner
- `repo` (string): Repository name
- `build` (number): Build number

**Example usage:**
```
get_build(owner="owner1", repo="repo1", build=123)
```

**Response:**
```
Build #123
Status: success
Ref: refs/heads/main
Commit: abc123def
Author: john.doe
Started: 2023-01-01 12:00:00 +0000 UTC
Event: push
Action: sync
```

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
├── main.go              # 主程序入口，处理命令行参数和服务器启动
├── tool/                # 工具处理模块
│   ├── build.go         # 构建相关工具（list_builds, get_build）
│   ├── repo.go          # 仓库相关工具（list_repos）
│   └── resource.go      # 资源处理（drone://builds/...）
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