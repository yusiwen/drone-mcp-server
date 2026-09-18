package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"drone-mcp-server/tool"
	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	useHTTP             = flag.Bool("http", false, "Use the Streamable HTTP transport instead of stdio")
	host                = flag.String("host", "localhost", "Host to listen on (HTTP mode only)")
	port                = flag.String("port", "8080", "Port to listen on (HTTP mode only)")
	path                = flag.String("path", "/", "Path of the MCP endpoint (HTTP mode only)")
	trustProxy          = flag.Bool("trust-proxy", false, "Trust X-Forwarded-For / X-Real-IP headers set by a reverse proxy")
	allowedHosts        = flag.String("allowed-hosts", "", "Comma-separated allowlist of accepted Host header values (HTTP mode only)")
	allowedOrigins      = flag.String("allowed-origins", "", "Comma-separated allowlist of accepted browser Origin values")
	localhostProtection = flag.Bool("localhost-protection", true, "Reject requests that arrive on a loopback address with a non-loopback Host header (DNS rebinding protection); an --allowed-hosts allowlist replaces this check unless this flag is set explicitly")
	enableWriteTools    = flag.Bool("enable-write-tools", false, "Register write and destructive tools (default: read-only tools only)")
	allowAnonymous      = flag.Bool("insecure-allow-anonymous", false, "Allow HTTP mode without MCP_AUTH_TOKEN (NOT recommended)")
	sessionTimeout      = flag.Duration("session-timeout", 30*time.Minute, "Idle MCP session timeout (HTTP mode only)")
	maxRequestBytes     = flag.Int64("max-request-bytes", 1<<20, "Maximum MCP request body size in bytes")
	version             = flag.Bool("version", false, "Show version information")
)

// Build-time variables (set via -ldflags)
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

const (
	// authFailureLimit is the number of failed authentication attempts
	// tolerated per client address within authFailureWindow.
	authFailureLimit  = 10
	authFailureWindow = time.Minute

	// minTokenLength is a weak sanity check on the configured bearer token.
	minTokenLength = 16

	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20
	shutdownTimeout   = 5 * time.Second

	// maxLogValueLength bounds attacker controlled values written to the log.
	maxLogValueLength = 200
)

// hiddenWriteTools counts tools that were skipped because write access is
// disabled. It is only written during registration.
var hiddenWriteTools int

type DroneServer struct {
	client          drone.Client
	repoHandler     *tool.RepoHandler
	buildHandler    *tool.BuildHandler
	resourceHandler *tool.ResourceHandler
	cronHandler     *tool.CronHandler
	secretHandler   *tool.SecretHandler
	userHandler     *tool.UserHandler
	templateHandler *tool.TemplateHandler
}

func main() {
	flag.Parse()

	// Show version information if requested
	if *version {
		fmt.Printf("drone-mcp-server\n")
		fmt.Printf("Version: %s\n", buildVersion)
		fmt.Printf("Commit: %s\n", buildCommit)
		fmt.Printf("Build date: %s\n", buildDate)
		fmt.Printf("Go version: %s\n", runtime.Version())
		os.Exit(0)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	droneServer := &DroneServer{}
	droneServer.initDroneClient()

	// Initialize handlers
	droneServer.repoHandler = tool.NewRepoHandler(droneServer.client)
	droneServer.buildHandler = tool.NewBuildHandler(droneServer.client)
	droneServer.resourceHandler = tool.NewResourceHandler(droneServer.client)
	droneServer.cronHandler = tool.NewCronHandler(droneServer.client)
	droneServer.secretHandler = tool.NewSecretHandler(droneServer.client)
	droneServer.userHandler = tool.NewUserHandler(droneServer.client)
	droneServer.templateHandler = tool.NewTemplateHandler(droneServer.client)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "drone-mcp-server",
		Version: buildVersion,
	}, nil)

	// Add logging middleware for MCP method calls
	server.AddReceivingMiddleware(loggingReceivingMiddleware())

	registerTools(server, droneServer)

	// Register resource template
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "Drone build details",
		Description: "Drone build details",
		MIMEType:    "text/plain",
		URITemplate: "drone://builds/{owner}/{repo}/{build}",
	}, droneServer.resourceHandler.HandleBuildResource)

	// Start server with the selected transport
	if *useHTTP {
		startHTTPServer(ctx, server)
		return
	}
	startStdioServer(ctx, server)
}

// validated is implemented by every tool argument struct.
type validated interface {
	Validate() error
}

// registerTool adds a tool to the server, rejecting arguments that fail the
// tool's own validation before the handler runs.
//
// Write and destructive tools are only registered when write access has been
// explicitly enabled, so a read-only deployment cannot be tricked into
// mutating the Drone instance at all.
func registerTool[In validated, Out any](server *mcp.Server, name, description string, annotations *mcp.ToolAnnotations, handler mcp.ToolHandlerFor[In, Out]) {
	if annotations != nil && !annotations.ReadOnlyHint && !writeToolsEnabled() {
		hiddenWriteTools++
		return
	}

	guarded := func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var zero Out
		if err := in.Validate(); err != nil {
			return nil, zero, fmt.Errorf("invalid arguments for %s: %w", name, err)
		}
		return handler(ctx, req, in)
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: annotations,
	}, guarded)
}

// readOnlyTool annotates a tool that does not modify the Drone instance.
func readOnlyTool() *mcp.ToolAnnotations {
	destructive := false
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: &destructive,
	}
}

// writeTool annotates a tool that modifies the Drone instance.
func writeTool(destructive, idempotent bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: &destructive,
		IdempotentHint:  idempotent,
	}
}

// writeToolsEnabled reports whether write tools may be registered. Write
// access must be requested explicitly, either with --enable-write-tools or
// with MCP_ENABLE_WRITE_TOOLS.
func writeToolsEnabled() bool {
	if *enableWriteTools {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MCP_ENABLE_WRITE_TOOLS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func registerTools(server *mcp.Server, s *DroneServer) {
	// Read-only tools
	registerTool(server, "list_repos", "List all repositories in Drone", readOnlyTool(), s.repoHandler.HandleListRepos)
	registerTool(server, "get_repo", "Get repository details", readOnlyTool(), s.repoHandler.HandleGetRepo)
	registerTool(server, "list_incomplete", "List repositories with incomplete builds", readOnlyTool(), s.repoHandler.HandleListIncomplete)
	registerTool(server, "list_crons", "List cron jobs for a repository", readOnlyTool(), s.cronHandler.HandleListCrons)
	registerTool(server, "get_cron", "Get cron job details", readOnlyTool(), s.cronHandler.HandleGetCron)
	registerTool(server, "list_secrets", "List repository secrets", readOnlyTool(), s.secretHandler.HandleListSecrets)
	registerTool(server, "get_secret", "Get repository secret details", readOnlyTool(), s.secretHandler.HandleGetSecret)
	registerTool(server, "list_org_secrets", "List organization secrets", readOnlyTool(), s.secretHandler.HandleListOrgSecrets)
	registerTool(server, "get_org_secret", "Get organization secret details", readOnlyTool(), s.secretHandler.HandleGetOrgSecret)
	registerTool(server, "get_self", "Get current authenticated user", readOnlyTool(), s.userHandler.HandleGetSelf)
	registerTool(server, "list_users", "List all users", readOnlyTool(), s.userHandler.HandleListUsers)
	registerTool(server, "get_user", "Get user details", readOnlyTool(), s.userHandler.HandleGetUser)
	registerTool(server, "list_templates", "List templates (optionally by namespace)", readOnlyTool(), s.templateHandler.HandleListTemplates)
	registerTool(server, "get_template", "Get template details and data", readOnlyTool(), s.templateHandler.HandleGetTemplate)
	registerTool(server, "list_builds", "List builds for a repository", readOnlyTool(), s.buildHandler.HandleListBuilds)
	registerTool(server, "get_build", "Get build details", readOnlyTool(), s.buildHandler.HandleGetBuild)
	registerTool(server, "get_build_last", "Get the last build for a repository (optionally by branch)", readOnlyTool(), s.buildHandler.HandleGetBuildLast)
	registerTool(server, "get_build_logs", "Get logs for a specific build stage and step", readOnlyTool(), s.buildHandler.HandleBuildLogs)

	// Write tools (registered only when write access is enabled)
	registerTool(server, "enable_repo", "Enable a repository", writeTool(false, true), s.repoHandler.HandleEnableRepo)
	registerTool(server, "disable_repo", "Disable a repository", writeTool(true, true), s.repoHandler.HandleDisableRepo)
	registerTool(server, "repair_repo", "Repair a repository", writeTool(false, true), s.repoHandler.HandleRepairRepo)
	registerTool(server, "chown_repo", "Change repository ownership", writeTool(true, false), s.repoHandler.HandleChownRepo)
	registerTool(server, "sync_repos", "Synchronize repository list", writeTool(false, true), s.repoHandler.HandleSyncRepos)

	registerTool(server, "create_cron", "Create a new cron job", writeTool(false, false), s.cronHandler.HandleCreateCron)
	registerTool(server, "delete_cron", "Delete a cron job", writeTool(true, true), s.cronHandler.HandleDeleteCron)
	registerTool(server, "execute_cron", "Execute a cron job immediately", writeTool(false, false), s.cronHandler.HandleExecuteCron)

	registerTool(server, "create_secret", "Create a repository secret", writeTool(false, false), s.secretHandler.HandleCreateSecret)
	registerTool(server, "update_secret", "Update a repository secret", writeTool(false, true), s.secretHandler.HandleUpdateSecret)
	registerTool(server, "delete_secret", "Delete a repository secret", writeTool(true, true), s.secretHandler.HandleDeleteSecret)
	registerTool(server, "create_org_secret", "Create an organization secret", writeTool(false, false), s.secretHandler.HandleCreateOrgSecret)
	registerTool(server, "update_org_secret", "Update an organization secret", writeTool(false, true), s.secretHandler.HandleUpdateOrgSecret)
	registerTool(server, "delete_org_secret", "Delete an organization secret", writeTool(true, true), s.secretHandler.HandleDeleteOrgSecret)

	registerTool(server, "create_user", "Create a new user", writeTool(false, false), s.userHandler.HandleCreateUser)
	registerTool(server, "update_user", "Update a user", writeTool(false, true), s.userHandler.HandleUpdateUser)
	registerTool(server, "delete_user", "Delete a user", writeTool(true, true), s.userHandler.HandleDeleteUser)

	registerTool(server, "create_template", "Create a new template", writeTool(false, false), s.templateHandler.HandleCreateTemplate)
	registerTool(server, "update_template", "Update a template", writeTool(false, true), s.templateHandler.HandleUpdateTemplate)
	registerTool(server, "delete_template", "Delete a template", writeTool(true, true), s.templateHandler.HandleDeleteTemplate)

	registerTool(server, "restart_build", "Restart a build (optionally with parameters)", writeTool(false, false), s.buildHandler.HandleRestartBuild)
	registerTool(server, "cancel_build", "Cancel a running build", writeTool(true, false), s.buildHandler.HandleCancelBuild)
	registerTool(server, "promote_build", "Promote a build to a target environment", writeTool(true, false), s.buildHandler.HandlePromoteBuild)
	registerTool(server, "rollback_build", "Rollback a deployment to a previous build", writeTool(true, false), s.buildHandler.HandleRollbackBuild)
	registerTool(server, "approve_build", "Approve a build stage (for gated deployments)", writeTool(false, false), s.buildHandler.HandleApproveBuild)
	registerTool(server, "decline_build", "Decline a build stage (for gated deployments)", writeTool(true, false), s.buildHandler.HandleDeclineBuild)
	registerTool(server, "create_build", "Create a new build from a commit or branch", writeTool(false, false), s.buildHandler.HandleCreateBuild)

	if hiddenWriteTools > 0 {
		log.Printf("Read-only mode: %d write/destructive tools are hidden. Start with --enable-write-tools (or MCP_ENABLE_WRITE_TOOLS=true) to register them.", hiddenWriteTools)
	}
}

// loggingReceivingMiddleware logs MCP method calls. Tool arguments are never
// logged, because they may contain secrets.
func loggingReceivingMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start := time.Now()

			toolName, isToolCall := toolCallName(method, req)
			if isToolCall {
				log.Printf("[TOOL] Tool called: %s", toolName)
			} else {
				log.Printf("[MCP] Method called: %s", method)
			}

			result, err := next(ctx, method, req)

			duration := time.Since(start)
			status := "completed successfully"
			if err != nil {
				status = fmt.Sprintf("failed: %v", err)
			}
			if isToolCall {
				log.Printf("[TOOL] Tool %s %s (took %v)", toolName, status, duration)
			} else {
				log.Printf("[MCP] Method %s %s (took %v)", method, status, duration)
			}
			return result, err
		}
	}
}

// toolCallName extracts the tool name from a tools/call request.
func toolCallName(method string, req mcp.Request) (string, bool) {
	if method != "tools/call" {
		return "", false
	}
	callReq, ok := req.(*mcp.CallToolRequest)
	if !ok || callReq.Params == nil {
		return "<unknown>", true
	}
	return callReq.Params.Name, true
}

func startStdioServer(ctx context.Context, server *mcp.Server) {
	log.Println("Starting MCP server with stdio transport...")
	log.Printf("Read-only mode is %s", readOnlyState())
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Fatalf("stdio server error: %v", err)
	}
}

func startHTTPServer(ctx context.Context, server *mcp.Server) {
	listenAddr := net.JoinHostPort(*host, *port)

	token, tokenSource, err := resolveAuthToken()
	if err != nil {
		log.Fatalf("authentication configuration error: %v", err)
	}

	allowedHostList := splitList(*allowedHosts)
	loopbackProtection, hostPolicyNote := hostPolicy(*localhostProtection, flagSet("localhost-protection"), allowedHostList)

	// The Streamable HTTP handler enables DNS rebinding protection for
	// requests that arrive on a loopback address and rejects cross-site
	// requests, as recommended by the MCP security best practices.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		// A single server instance is shared by all sessions.
		return server
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout:      *sessionTimeout,
		MaxRequestBodyBytes: *maxRequestBytes,
		// The default (false) rejects requests that arrive on a loopback
		// address with a non-loopback Host header, which is the DNS
		// rebinding defense recommended by the MCP security best
		// practices.
		DisableLocalhostProtection: !loopbackProtection,
	})

	var handler http.Handler = mcpHandler

	if hostPolicyNote != "" {
		log.Println(hostPolicyNote)
	}

	// Cross-site request forgery protection for browser based clients.
	// Requests without Origin/Sec-Fetch-Site (that is, non-browser clients)
	// are unaffected.
	originProtection := http.NewCrossOriginProtection()
	for _, origin := range splitList(*allowedOrigins) {
		if err := originProtection.AddTrustedOrigin(origin); err != nil {
			log.Fatalf("invalid --allowed-origins entry %q: %v", origin, err)
		}
	}
	handler = originProtection.Handler(handler)

	// Optional Host allowlist. It replaces the transport's loopback
	// heuristic unless --localhost-protection was set explicitly.
	handler = hostGuard(handler, allowedHostList)

	if token == "" {
		if !*allowAnonymous {
			log.Fatal("refusing to start HTTP mode without MCP_AUTH_TOKEN: set MCP_AUTH_TOKEN (or MCP_AUTH_TOKEN_FILE), or pass --insecure-allow-anonymous to accept unauthenticated access")
		}
		log.Println("WARNING: HTTP mode is running without authentication (--insecure-allow-anonymous); anyone who can reach this port can use the configured Drone token")
	} else {
		if len(token) < minTokenLength {
			log.Printf("WARNING: MCP_AUTH_TOKEN is shorter than %d characters and may be brute forced", minTokenLength)
		}
		handler = authMiddleware(handler, token, newAuthLimiter(authFailureLimit, authFailureWindow))
		log.Printf("HTTP authentication enabled (bearer token from %s)", tokenSource)
	}

	handler = loggingMiddleware(handler)

	// Ensure path starts with "/"
	endpointPath := *path
	if !strings.HasPrefix(endpointPath, "/") {
		endpointPath = "/" + endpointPath
	}

	mux := http.NewServeMux()
	mux.Handle(endpointPath, handler)

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// ReadTimeout and WriteTimeout stay unset so that long lived
		// streaming responses are not cut off; slow request bodies are
		// bounded by MaxRequestBodyBytes instead.
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP shutdown error: %v", err)
		}
	}()

	log.Printf("Starting MCP server with Streamable HTTP transport on http://%s%s", listenAddr, endpointPath)
	log.Printf("MCP endpoint: POST/GET/DELETE %s", endpointPath)
	log.Printf("Read-only mode is %s", readOnlyState())
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server error: %v", err)
	}
	log.Println("HTTP server stopped")
}

// readOnlyState describes the tool registration mode for log output.
func readOnlyState() string {
	if writeToolsEnabled() {
		return "disabled (write tools are registered)"
	}
	return "enabled (write tools are hidden)"
}

// resolveAuthToken returns the configured bearer token and a human readable
// description of where it came from. An empty token means authentication is
// not configured.
func resolveAuthToken() (string, string, error) {
	if file := strings.TrimSpace(os.Getenv("MCP_AUTH_TOKEN_FILE")); file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return "", "", fmt.Errorf("cannot read MCP_AUTH_TOKEN_FILE: %w", err)
		}
		token := strings.TrimSpace(string(raw))
		if token == "" {
			return "", "", fmt.Errorf("MCP_AUTH_TOKEN_FILE %q is empty", file)
		}
		return token, "MCP_AUTH_TOKEN_FILE", nil
	}
	if token := strings.TrimSpace(os.Getenv("MCP_AUTH_TOKEN")); token != "" {
		return token, "MCP_AUTH_TOKEN", nil
	}
	return "", "", nil
}

// authLimiter throttles repeated authentication failures per client address.
type authLimiter struct {
	mu       sync.Mutex
	attempts map[string]*authAttempt
	limit    int
	window   time.Duration
	now      func() time.Time
}

type authAttempt struct {
	count int
	first time.Time
}

func newAuthLimiter(limit int, window time.Duration) *authLimiter {
	return &authLimiter{
		attempts: make(map[string]*authAttempt),
		limit:    limit,
		window:   window,
		now:      time.Now,
	}
}

// blocked reports whether the client address has exhausted its attempts.
func (l *authLimiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt, ok := l.attempts[key]
	if !ok {
		return false
	}
	if l.now().Sub(attempt.first) > l.window {
		delete(l.attempts, key)
		return false
	}
	return attempt.count >= l.limit
}

// failure records a failed authentication attempt.
func (l *authLimiter) failure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	attempt, ok := l.attempts[key]
	if !ok || now.Sub(attempt.first) > l.window {
		l.attempts[key] = &authAttempt{count: 1, first: now}
		return
	}
	attempt.count++
}

// success clears the failure counter after a valid authentication.
func (l *authLimiter) success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// authMiddleware validates a bearer token in constant time and throttles
// repeated failures.
func authMiddleware(next http.Handler, expectedToken string, limiter *authLimiter) http.Handler {
	expectedHash := sha256.Sum256([]byte(expectedToken))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := clientIP(r, *trustProxy)
		requestPath := sanitizeLog(r.URL.EscapedPath())

		if limiter.blocked(client) {
			log.Printf("[AUTH] Rate limited %s %s (client: %s)", r.Method, requestPath, sanitizeLog(client))
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many failed authentication attempts", http.StatusTooManyRequests)
			return
		}

		WWWAuthenticate := `Bearer realm="drone-mcp-server"`

		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			limiter.failure(client)
			log.Printf("[AUTH] Bearer token missing or malformed on %s %s (client: %s)", r.Method, requestPath, sanitizeLog(client))
			w.Header().Set("WWW-Authenticate", WWWAuthenticate)
			http.Error(w, "Authorization header with a Bearer token is required", http.StatusUnauthorized)
			return
		}

		providedHash := sha256.Sum256([]byte(token))
		if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
			limiter.failure(client)
			log.Printf("[AUTH] Invalid bearer token on %s %s (client: %s)", r.Method, requestPath, sanitizeLog(client))
			w.Header().Set("WWW-Authenticate", WWWAuthenticate)
			http.Error(w, "Invalid bearer token", http.StatusUnauthorized)
			return
		}

		limiter.success(client)
		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the credentials of a "Bearer <token>" header. The
// scheme is matched case-insensitively, as required by RFC 7235.
func bearerToken(header string) (string, bool) {
	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

// flagSet reports whether the named flag was provided on the command line.
// It distinguishes an explicit --localhost-protection=true from the default
// value.
func flagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// hostPolicy selects the Host header validation strategy.
//
// An --allowed-hosts allowlist replaces the transport's loopback heuristic, so
// that a reverse proxy deployment needs one flag instead of two. The two checks
// are only combined when --localhost-protection was set explicitly. The
// returned note is logged when it explains a non-obvious decision.
func hostPolicy(localhostProtection, explicitlySet bool, allowedHosts []string) (loopbackCheck bool, note string) {
	if len(allowedHosts) > 0 && !explicitlySet {
		return false, "Host allowlist configured: using --allowed-hosts instead of the transport's loopback Host heuristic"
	}
	if !localhostProtection {
		if len(allowedHosts) == 0 {
			return false, "WARNING: loopback DNS rebinding protection is disabled and no --allowed-hosts allowlist is configured; any Host header is accepted"
		}
		return false, "Loopback DNS rebinding protection is disabled; the --allowed-hosts allowlist is the compensating control"
	}
	return true, ""
}

// hostGuard rejects requests whose Host header is not in the allowlist.
// An empty allowlist disables the check.
func hostGuard(next http.Handler, allowed []string) http.Handler {
	if len(allowed) == 0 {
		return next
	}
	normalized := make(map[string]struct{}, len(allowed))
	for _, entry := range allowed {
		if host := normalizeHost(entry); host != "" {
			normalized[host] = struct{}{}
		}
	}
	if len(normalized) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := normalized[normalizeHost(r.Host)]; !ok {
			log.Printf("[SECURITY] Rejected request with disallowed Host header %q from %s", sanitizeLog(r.Host), sanitizeLog(clientIP(r, *trustProxy)))
			http.Error(w, "Forbidden: host not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// normalizeHost lowercases a host and strips its port.
func normalizeHost(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return strings.Trim(value, "[]")
}

// loggingMiddleware logs access information with attacker controlled values
// sanitized so that log entries cannot be forged.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// The escaped path is logged: the decoded path may contain newlines.
		requestPath := sanitizeLog(r.URL.EscapedPath())
		client := sanitizeLog(clientIP(r, *trustProxy))
		userAgent := sanitizeLog(r.Header.Get("User-Agent"))
		if userAgent == "" {
			userAgent = "-"
		}
		contentType := sanitizeLog(r.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "-"
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		log.Printf("[ACCESS] %s %s %s %d %s %s %s",
			client,
			r.Method,
			requestPath,
			rw.statusCode,
			contentType,
			userAgent,
			time.Since(start),
		)
	})
}

// sanitizeLog removes control characters from a value before it is logged and
// truncates it, so that request data cannot inject forged log lines.
func sanitizeLog(value string) string {
	if value == "" {
		return value
	}
	cleaned := strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	if runes := []rune(cleaned); len(runes) > maxLogValueLength {
		cleaned = string(runes[:maxLogValueLength]) + "...(truncated)"
	}
	return cleaned
}

// splitList splits a comma separated flag value, dropping empty entries.
func splitList(value string) []string {
	var out []string
	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			out = append(out, entry)
		}
	}
	return out
}

// responseWriter wraps http.ResponseWriter to capture status code and support streaming interfaces
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	hijacked   bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.hijacked {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.hijacked {
		return 0, http.ErrHijacked
	}
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for streaming responses
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker for connection upgrades
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		rw.hijacked = true
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// Push implements http.Pusher for HTTP/2 server push
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// ReadFrom implements io.ReaderFrom for efficient copying
func (rw *responseWriter) ReadFrom(r io.Reader) (n int64, err error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	if rf, ok := rw.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(rw.ResponseWriter, r)
}

// clientIP returns the client address of a request. Forwarding headers are
// only honored when the server is explicitly told to trust a reverse proxy,
// because they are otherwise attacker controlled.
func clientIP(r *http.Request, trustForwardedHeaders bool) string {
	if trustForwardedHeaders {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			if first, _, _ := strings.Cut(forwarded, ","); strings.TrimSpace(first) != "" {
				return strings.TrimSpace(first)
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *DroneServer) initDroneClient() {
	serverURL := os.Getenv("DRONE_SERVER")
	token := os.Getenv("DRONE_TOKEN")

	if serverURL == "" || token == "" {
		log.Fatal("DRONE_SERVER and DRONE_TOKEN environment variables must be set")
	}

	// Create HTTP client with token authentication
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &tokenTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}

	s.client = drone.NewClient(serverURL, httpClient)
}

// tokenTransport adds Authorization header to requests
type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}
