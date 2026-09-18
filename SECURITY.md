# Security Policy

## Supported versions

Security fixes are applied to the latest tagged release and to `main`.

## Reporting a vulnerability

Please do not open a public issue. Use GitHub's private vulnerability reporting (the "Report a vulnerability" button on the repository's Security tab) and include:

- the affected version or commit,
- a minimal reproduction,
- the impact you believe it has,
- any suggested fix.

You can expect an initial response within a few days. Credit is given in the release notes unless you ask to stay anonymous.

## Threat model

`drone-mcp-server` is a privileged proxy in front of a Drone instance: it holds a Drone API token and exposes it through MCP tools. Whoever can reach the server and authenticate acts with the full authority of that token.

### Assets

| Asset | Notes |
| :--- | :--- |
| `DRONE_TOKEN` | Drone personal access token used for every Drone API call. Frequently administrator scoped, because the tool set includes user and organization management. |
| `MCP_AUTH_TOKEN` | Bearer token gating the MCP endpoint. Grants access to every registered tool. |
| Drone resources | Repositories, builds, cron jobs, secret metadata, users and templates reachable with `DRONE_TOKEN`. |

### Trust boundaries

1. **MCP client to server.** In stdio mode the client runs locally and is trusted. In HTTP mode the network boundary is untrusted, so authentication, `Host`/`Origin` validation and request limits apply.
2. **Server to Drone.** The Drone API is trusted, but its responses are treated as untrusted data that is forwarded to the client.
3. **Model context.** Build logs, template bodies, repository names and user names are attacker-influenceable content that reaches a large language model. They can carry prompt injection payloads, which is why write tools are disabled by default.

### In scope

- Authentication bypass, weak token handling or throttling bypass on the HTTP transport
- DNS rebinding or cross-site requests against the HTTP transport
- Request path or query injection into the Drone API through tool arguments
- Disclosure of `DRONE_TOKEN`, `MCP_AUTH_TOKEN` or Drone secret values through logs, errors or tool output
- Log forging or audit evasion
- Privilege escalation through the tool surface (for example, a read-only deployment somehow granting write access)
- Dependency or CI/CD supply chain issues in this repository

### Out of scope

- Anyone holding `MCP_AUTH_TOKEN` or `DRONE_TOKEN` intentionally calling tools
- Vulnerabilities in Drone itself or in the MCP Go SDK (report those upstream)
- An attacker who can already read the server's environment variables or token file
- Denial of service by an authenticated client that stays inside the configured request limits
- Missing rate limits on Drone API calls (the server is a thin proxy)

## Security controls

| Control | Implementation |
| :--- | :--- |
| Authentication | Bearer token from `MCP_AUTH_TOKEN` / `MCP_AUTH_TOKEN_FILE`, compared in constant time over SHA-256 digests, with per-address failure throttling |
| Fail closed | HTTP mode refuses to start without a token unless `--insecure-allow-anonymous` is passed |
| Transport | `mcp.StreamableHTTPHandler` with loopback DNS rebinding protection, cross-origin protection, idle session timeout and a request body cap |
| Host / Origin allowlists | `--allowed-hosts` and `--allowed-origins` |
| Tool surface | 18 read-only tools by default, 27 write tools behind `--enable-write-tools`, MCP annotations on every tool |
| Input validation | `tool/validate.go` plus a per-tool `Validate()` enforced by `registerTool` |
| Logging | Forwarded headers ignored unless `--trust-proxy`, control characters stripped, tool arguments never logged |
| Supply chain | GitHub Actions pinned by commit SHA, `govulncheck` gate, Dependabot for Go modules and actions, container scan fails the build on critical findings |

## Deployment requirements

1. Terminate TLS in front of the server; the binary speaks plain HTTP.
2. Do not expose the port publicly without authentication - publish it on a loopback or private interface.
3. Use a high-entropy `MCP_AUTH_TOKEN` (32+ random bytes) delivered as a platform secret.
4. Keep write tools disabled unless they are needed, and grant `DRONE_TOKEN` only the permissions the enabled tools require.
5. Set `--trust-proxy` only when a proxy you control rewrites the forwarding headers.
